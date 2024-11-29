package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	mydb "myDatabase"
)

var ErrTooFewDays error = errors.New("too few days")
var ErrNotInsterested error = errors.New("not interested")

const BASE_QDS_NR int = 19
const MIN_INTRESTED_VOL int64 = 300000 // qty 300,000

const MAX_TRANSMIT_SIZE int = 32

const TYPE_GAP_CALL int = 1
const TYPE_GAP_PUT int = 2
const TYPE_GAP_CALL_CLOSED int = 3
const TYPE_GAP_PUT_CLOSED int = 4

func TypeToStr(t int) string {
	switch t {
	case TYPE_GAP_CALL:
		return "Call"
	case TYPE_GAP_PUT:
		return "Put"
	case TYPE_GAP_CALL_CLOSED:
		return "CallClosed"
	case TYPE_GAP_PUT_CLOSED:
		return "PutClosed"
	}
	return ""
}

type Result struct {
	Config interface{} `json:"config"`
	Info   string      `json:"info,omitempty"`
}

type ScanRequest struct {
	Op       string `json:"op"`
	Interval int    `json:"interval"`
	Next     int    `json:"next"`
}

type Reply struct {
	Result     []Result `json:"result"`
	NextTblIdx int      `json:"next"`
}

func isWeekend(t time.Time) bool {
	switch t.Weekday() {
	case time.Saturday:
		return true
	case time.Sunday:
		return true
	default:
		return false
	}
}

func doScan(w http.ResponseWriter, r *http.Request) {
	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrResonse(w, err.Error(), http.StatusBadRequest)
		return
	}

	interval := req.Interval
	op := req.Op
	next := req.Next
	fmt.Printf("Start scan.. op=%s interval=%d next=%d\n", op, interval, next)

	err := updateFetch()
	if err != nil {
		writeJSONErrResonse(w, err.Error(), http.StatusInternalServerError)
		fmt.Println(err.Error())
		return
	}

	continueScan(w, next, op, interval)
}

func continueScan(w http.ResponseWriter, tblIdx int, op string, interval int) {
	reply := Reply{}

	if tblIdx < 0 {
		writeJSONErrResonse(w, "Invalid table index", http.StatusBadRequest)
		fmt.Println("Invalid table index", tblIdx)
		return
	}

	tables := mydb.GetTableList()
	if tables == nil {
		writeJSONErrResonse(w, "Failed to get table list", http.StatusInternalServerError)
		fmt.Println("Failed to get table list")
		return
	}

	var result Result
	var foundNr int = 0
	for i := tblIdx; i < len(tables); i = i + 1 {
		tblName := tables[i]
		fmt.Printf("Getting DQ for %s...\r", tblName)

		dqs, err := mydb.GetDailyQuote(tblName, interval+BASE_QDS_NR)
		if err != nil {
			writeJSONErrResonse(w, err.Error(), http.StatusInternalServerError)
			fmt.Println("Failed to get DQ:", err.Error())
			return
		}

		switch op {
		case "gap":
			result, err = findGap(tblName, interval, dqs)
		case "vol-burst":
			result, err = findVolBurst(tblName, interval, dqs)
		default:
			writeJSONErrResonse(w, "No such op code", http.StatusBadRequest)
			fmt.Println("No such op code", op)
			return
		}

		if err == ErrNotInsterested || err == ErrTooFewDays {
			continue
		}
		if err != nil {
			writeJSONErrResonse(w, err.Error(), http.StatusInternalServerError)
			fmt.Println("Failed execute", op, err.Error())
			return
		}

		fmt.Printf("Found candidate %s+\n", tblName)
		reply.Result = append(reply.Result, result)
		foundNr += 1

		// TEMP
		// if foundNr > 0 {
		// 	reply.NextTblIdx = -1
		// 	writeJSONOKResonse(w, reply)
		// 	return
		// }

		if foundNr > MAX_TRANSMIT_SIZE {
			reply.NextTblIdx = i + 1
			writeJSONOKResonse(w, reply)
			return
		}
	}

	reply.NextTblIdx = -1
	writeJSONOKResonse(w, reply)
}

// Date in ascendent
func genMA(dqs []mydb.DaliyQuote, maNr int) []DataPoint {
	ma := []DataPoint{}
	sum := 0.0
	i := BASE_QDS_NR + 1 - maNr

	// fmt.Println("CheckingMA", maNr)

	for i < BASE_QDS_NR {
		sum += dqs[i].Close
		// fmt.Printf("[%d] sum=%f\n", i, sum)
		i += 1
	}

	for i < len(dqs) {
		sum += dqs[i].Close
		val := sum / float64(maNr)
		mmdd := fmt.Sprintf("%02d%02d", dqs[i].Month, dqs[i].Day)
		ma = append(ma, GenXYDataPoint(mmdd, val))
		// fmt.Printf("[%d] sum=%f(%f) ma=%f\n", i, sum, dqs[i].Close, ma)
		sum -= dqs[i-maNr+1].Close
		// fmt.Printf("sum=%f(%f)\n", sum, dqs[i-maNr+1].Close)
		i += 1
	}
	return ma
}

func toMMDD(dq *mydb.DaliyQuote) string {
	return fmt.Sprintf("%02d%02d", dq.Month, dq.Day)
}
func toCandleDataPoint(mmdd string, dq *mydb.DaliyQuote) DataPoint {
	return GenCandleDataPoint(mmdd, dq.Open, dq.High, dq.Low, dq.Close)
}
func toVolumeDataPoint(mmdd string, dq *mydb.DaliyQuote) DataPoint {
	return GenXYDataPoint(mmdd, float64(dq.Volume/1000))
}
