package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	mydb "myDatabase"
)

var ErrTooFewDays error = errors.New("too few days")
var ErrNotInsterested error = errors.New("not interested")

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

type DQCandle struct {
	Date  string  `json:"date"`
	Open  float64 `json:"open"`
	High  float64 `json:"high"`
	Low   float64 `json:"low"`
	Close float64 `json:"close"`
}

type DQPlot struct {
	Code   string     `json:"code"`
	Type   int        `json:"type"`
	Candle []DQCandle `json:"candle"`
	Ma5    []float64  `json:"ma5"`
	Ma10   []float64  `json:"ma10"`
	Ma20   []float64  `json:"ma20"`
	HLine  []float64  `json:"line"`
}

type ScanRequest struct {
	Op       string `json:"op"`
	Interval int    `json:"interval"`
	Next     int    `json:"next"`
}

type ScnaReply struct {
	Data       []DQPlot `json:"data"`
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

	err := updateFetch(interval)
	if err != nil {
		writeJSONErrResonse(w, err.Error(), http.StatusInternalServerError)
		fmt.Println(err.Error())
		return
	}

	continueScan(w, next, op, interval)
}

func continueScan(w http.ResponseWriter, tblIdx int, op string, interval int) {
	reply := ScnaReply{}

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

	var plot DQPlot
	var foundNr int = 0
	for i := tblIdx; i < len(tables); i = i + 1 {
		tblName := tables[i]
		fmt.Printf("Getting DQ for %s...\r", tblName)

		dqs, err := mydb.GetDailyQuote(tblName, interval)
		if err != nil {
			writeJSONErrResonse(w, err.Error(), http.StatusInternalServerError)
			fmt.Println("Failed to get DQ:", err.Error())
			return
		}

		switch op {
		case "gap":
			plot, err = findGap(tblName, dqs)
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
		reply.Data = append(reply.Data, plot)
		foundNr += 1

		// TEMP
		if foundNr > 0 {
			reply.NextTblIdx = -1
			writeJSONOKResonse(w, reply)
			return
		}

		if foundNr > MAX_TRANSMIT_SIZE {
			reply.NextTblIdx = i + 1
			writeJSONOKResonse(w, reply)
			return
		}
	}

	reply.NextTblIdx = -1
	writeJSONOKResonse(w, reply)
}

func genMA(dqs []mydb.DaliyQuote, interval int) []float64 {
	day := len(dqs) - 1
	ma := []float64{}
	sum := 0.0

	// fmt.Println("CheckingMA", interval)

	prev := day - interval + 1
	for day > prev {
		sum += dqs[day].Close
		// fmt.Printf("day=%d sum=%f\n", day, sum)
		day -= 1
	}

	for day >= 0 {
		sum += dqs[day].Close
		ma = append(ma, sum/float64(interval))
		// fmt.Printf("2day=%d sum=%f(%f) ma=%f\n", day, sum, dqs[day].Close, ma)
		sum -= dqs[day+interval-1].Close
		// fmt.Printf("sum=%f(%f)\n", sum, dqs[day+interval-1].Close)
		day -= 1
	}
	return ma
}

func genCandle(dq mydb.DaliyQuote) DQCandle {
	candle := DQCandle{Date: fmt.Sprintf("%04d%02d%02d", dq.Year, dq.Month, dq.Day), Open: dq.Open, High: dq.High, Low: dq.Low, Close: dq.Close}
	return candle
}
func genPlot(code string, ftype int, candle []DQCandle, ma5 []float64, ma10 []float64, ma20 []float64, hline []float64) DQPlot {
	var plot DQPlot
	plot.Code = code
	plot.Type = ftype
	plot.Candle = candle
	plot.Ma5 = ma5
	plot.Ma10 = ma10
	plot.Ma20 = ma20
	plot.HLine = hline
	return plot
}

func findGap(code string, dqs []mydb.DaliyQuote) (DQPlot, error) {
	day := len(dqs) - 1
	if day < 19 {
		fmt.Println("ERR:", code, "days is", day)
		return DQPlot{}, ErrTooFewDays
	}

	findings := 0
	candle := []DQCandle{}
	hline := []float64{}

	for day > 0 {
		dq1 := dqs[day]   // former day
		dq2 := dqs[day-1] // current loop day

		h1 := math.Max(dq1.Open, dq1.Close)
		h2 := math.Max(dq2.Open, dq2.Close)

		l1 := math.Min(dq1.Open, dq1.Close)
		l2 := math.Min(dq2.Open, dq2.Close)

		highGap := l2 - h1
		lowGap := h2 - l1
		const GAP_MUL float64 = 1.015

		if findings != TYPE_GAP_CALL && highGap > 0 {
			if h1*GAP_MUL < l2 {
				findings = TYPE_GAP_CALL
				hline = []float64{h1, l2}
			}
		}
		if findings != TYPE_GAP_PUT && lowGap > 0 {
			if l1 > h2*GAP_MUL {
				findings = TYPE_GAP_PUT
				hline = []float64{l1, h2}
			}
		}

		candle = append(candle, genCandle(dq1))
		day -= 1
	}
	candle = append(candle, genCandle(dqs[0]))

	if findings == 0 {
		return DQPlot{}, ErrNotInsterested
	}

	ma5 := genMA(dqs, 5)
	ma10 := genMA(dqs, 10)
	ma20 := genMA(dqs, 20)

	avg5 := ma5[len(ma5)-1]
	avg10 := ma10[len(ma10)-1]
	avg20 := ma20[len(ma20)-1]

	if findings == TYPE_GAP_CALL {
		fail5 := dqs[0].Close <= avg5
		fail10 := dqs[0].Close <= avg10
		fail20 := dqs[0].Close <= avg20
		if fail5 && fail10 && fail20 {
			findings = TYPE_GAP_CALL_CLOSED
		}
	}
	if findings == TYPE_GAP_PUT {
		fail5 := dqs[0].Close >= avg5
		fail10 := dqs[0].Close >= avg10
		fail20 := dqs[0].Close >= avg20
		if fail5 && fail10 && fail20 {
			findings = TYPE_GAP_PUT_CLOSED
		}
	}

	plot := genPlot(code, findings, candle, ma5, ma10, ma20, hline)
	return plot, nil
}
