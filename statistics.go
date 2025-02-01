package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"time"

	mydb "myDatabase"
)

type StatisRequest struct {
	Op       string `json:"op"`
	Interval int    `json:"interval"`
}
type StatisReply struct {
	Result       []Result `json:"result"`
	NextTblIdx   int      `json:"next"`
	MarketNets   []int64  `json:"marketnets,omitempty"`
	Values       int64    `json:"values,omitempty"`
	MarketValues int64    `json:"marketvals,omitempty"`
}
type OldReply struct {
	Labels []string `json:"labels"`
	Data   []int    `json:"data"`
}

func doStatistic(w http.ResponseWriter, r *http.Request) {
	var req StatisRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrResonse(w, err.Error(), http.StatusBadRequest)
	}

	switch req.Op {
	case "init":
		err := genAllTimeGain()
		if err != nil {
			writeJSONErrResonse(w, err.Error(), http.StatusBadRequest)
		}
		writeJSONOKResonse(w, map[string]string{})
	case "gain":
		reply, err := calGain(req.Interval)
		if err != nil {
			writeJSONErrResonse(w, err.Error(), http.StatusBadRequest)
		}
		writeJSONOKResonse(w, reply)
	case "holding":
		getHolding(w)
	}
}

func genAllTimeGain() error {
	err := mydb.ResetTbl(mydb.HOLDING_TABLENAME)
	if err != nil {
		log.Fatalln("errResetHolding", err.Error())
	}
	err = mydb.ResetTbl(mydb.REALIZED_TABLENAME)
	if err != nil {
		log.Fatalln("errResetRealized", err.Error())
	}
	err = mydb.VacuumDB()
	if err != nil {
		log.Fatalln("errVacuum", err.Error())
	}

	trans, err := mydb.ScanTransaction()
	if err != nil {
		fmt.Println("Some error ", err.Error())
		return err
	}

	totalNr := len(trans)
	for i, v := range trans {
		err = procTrans(v)
		if err != nil {
			fmt.Println("Failed")
			return err
		}
		fmt.Printf("\rProgress:%d/%d", i, totalNr)
	}
	fmt.Println("Complete")
	return nil
}

func procTrans(v mydb.Transaction) error {
	if v.Direction {
		// Buy
		h := mydb.Holding{Code: v.Code, Year: v.Year, Month: v.Month, Day: v.Day, Quantity: v.Quantity, Net: v.Net}
		mydb.AddHolding(h)
		return nil
	}

	// Sell
	holdings, err := mydb.GetHolding(v.Code)
	if err != nil {
		fmt.Println("Error for scan", v.Code, err.Error())
		return err
	}

	remain := v.Quantity
	remainNet := v.Net
	gain := 0
	for _, h := range holdings {
		nr, err := mydb.DecHolding(h, remain)
		if err != nil {
			fmt.Println("Some error for dec", v.Code, v.Year, v.Month, v.Day, err.Error())
			return err
		}

		// v should be selling. h should be bought holdings
		hRatio := float64(nr) / float64(h.Quantity)
		// fmt.Printf("hRatio %f=%d/%d\n", hRatio, nr, h.Quantity)
		vRatio := float64(nr) / float64(remain)
		// fmt.Printf("vRatio %f=%d/%d\n", vRatio, nr, remain)
		vUsed := int(math.Round(float64(remainNet) * vRatio))
		// fmt.Printf("vUsed %d=%d*%f\n", vUsed, remainNet, vRatio)
		gain += vUsed - int(math.Round(float64(h.Net)*hRatio))
		// fmt.Printf("gan=%d=%d-(%d*%f)\n", gain, vUsed, h.Net, hRatio)
		remain -= nr
		remainNet -= vUsed
		if remain == 0 {
			break
		}
	}
	if remain != 0 {
		fmt.Printf("Remaining for code=%s at %d/%d/%d... May be missing buy info.\n", v.Code, v.Year, v.Month, v.Day)
		if remain == v.Quantity {
			// No need to add realized
			return err
		}
	}

	realized := mydb.Holding{Code: v.Code, Year: v.Year, Month: v.Month, Day: v.Day, Quantity: (v.Quantity - remain), Net: gain}
	err = mydb.AddRealized(realized)
	if err != nil {
		fmt.Println("Error for add realized", v.Code, v.Year, v.Month, v.Day, err.Error())
		return err
	}
	return nil
}

func calGain(interval int) (reply OldReply, err error) {
	y, d, m := time.Now().AddDate(0, -interval, 0).Date()
	realizeds, err := mydb.GetRelized(y, int(d), m)
	if err != nil {
		return reply, err
	}

	rmap := make(map[string]int)
	for _, ent := range realizeds {
		if val, exist := rmap[ent.Code]; exist {
			rmap[ent.Code] = val + ent.Net
		} else {
			rmap[ent.Code] = ent.Net
		}
	}

	keys := make([]string, 0, len(rmap))
	for k := range rmap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		val := rmap[k]
		name, err := mydb.RefLookupNameByCode(k)
		if err != nil {
			return reply, err
		}
		reply.Labels = append(reply.Labels, k+name)
		reply.Data = append(reply.Data, val)
	}
	return reply, nil
}

func getHolding(w http.ResponseWriter) {
	labels := []string{}
	bgColor := []string{}
	nets := []float64{}
	marketNets := []int64{}
	prev := mydb.Holding{}
	holdingValues := 0
	marketValues := 0.0

	holdings, err := mydb.GetHoldingAll()
	if err != nil {
		return
	}

	for i, ent := range holdings {
		if i == 0 {
			prev = ent
			continue
		}
		if ent.Code == prev.Code {
			// fmt.Printf("add %v + %v\n", prev, ent)
			prev.Quantity += ent.Quantity
			prev.Net += ent.Net
		} else {
			// fmt.Printf("== %v\n", prev)
			name, err := mydb.RefLookupNameByCode(prev.Code)
			if err != nil {
				writeJSONErrResonse(w, "Code-Name pair not found", http.StatusInternalServerError)
				return
			}

			var mknet float64 = 0.0
			dq, err := mydb.GetDailyQuote(mydb.STKPREFIX+prev.Code, 1)
			if err == nil {
				mknet = dq[0].Close * float64(prev.Quantity)
				marketValues += mknet
			}

			holdingValues += prev.Net

			labels = append(labels, prev.Code+name)
			nets = append(nets, float64(prev.Net))
			marketNets = append(marketNets, int64(mknet))
			bgColor = append(bgColor, GenBGColor())
			prev = ent
		}
	}
	// fmt.Printf("== %v\n", prev)

	var mknet float64 = 0.0
	dq, err := mydb.GetDailyQuote(mydb.STKPREFIX+prev.Code, 1)
	if err == nil {
		mknet = dq[0].Close*float64(prev.Quantity) - float64(prev.Net)
		marketValues += mknet
	}

	name, err := mydb.RefLookupNameByCode(prev.Code)
	if err != nil {
		writeJSONErrResonse(w, "Code-Name pair not found", http.StatusInternalServerError)
		return
	}
	holdingValues += prev.Net
	labels = append(labels, prev.Code+name)
	nets = append(nets, float64(prev.Net))
	marketNets = append(marketNets, int64(mknet))
	bgColor = append(bgColor, GenBGColor())

	ds := GenGenericDataset("doughnut", "Holdings", nets, bgColor)
	config := GenGenericChartConfig("doughnut", labels, []GenericDataset{ds})

	res := Result{Config: config}
	reply := StatisReply{Result: []Result{res}, NextTblIdx: 0, MarketNets: marketNets, Values: int64(holdingValues), MarketValues: int64(marketValues)}

	writeJSONOKResonse(w, reply)
}
