package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	mydb "myDatabase"
)

var ErrTooFewDays error = errors.New("too few days")
var ErrNotInsterested error = errors.New("not interested")

const BASE_QDS_NR int = 19

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

	err := updateFetch()
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

		dqs, err := mydb.GetDailyQuote(tblName, interval+BASE_QDS_NR)
		if err != nil {
			writeJSONErrResonse(w, err.Error(), http.StatusInternalServerError)
			fmt.Println("Failed to get DQ:", err.Error())
			return
		}

		switch op {
		case "gap":
			plot, err = findGap(tblName, interval, dqs)
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
func genMA(dqs []mydb.DaliyQuote, maNr int) []float64 {
	ma := []float64{}
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
		ma = append(ma, sum/float64(maNr))
		// fmt.Printf("[%d] sum=%f(%f) ma=%f\n", i, sum, dqs[i].Close, ma)
		sum -= dqs[i-maNr+1].Close
		// fmt.Printf("sum=%f(%f)\n", sum, dqs[i-maNr+1].Close)
		i += 1
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

func findGapCallClose(interval int, foundDay int, dqs []mydb.DaliyQuote, ma5 []float64, ma10 []float64, ma20 []float64) int {
	for i := foundDay + 1; i < interval+BASE_QDS_NR; i = i + 1 {
		maDay := i - BASE_QDS_NR
		avg5 := ma5[maDay]
		avg10 := ma10[maDay]
		avg20 := ma20[maDay]

		fail5 := dqs[i].Close <= avg5
		fail10 := dqs[i].Close <= avg10
		fail20 := dqs[i].Close <= avg20

		if fail5 && fail10 && fail20 {
			return i
		}
	}
	return -1
}

func findGapPutClose(interval int, foundDay int, dqs []mydb.DaliyQuote, ma5 []float64, ma10 []float64, ma20 []float64) int {
	for i := foundDay + 1; i < interval+BASE_QDS_NR; i = i + 1 {
		maDay := i - BASE_QDS_NR
		avg5 := ma5[maDay]
		avg10 := ma10[maDay]
		avg20 := ma20[maDay]

		fail5 := dqs[i].Close >= avg5
		fail10 := dqs[i].Close >= avg10
		fail20 := dqs[i].Close >= avg20

		if fail5 && fail10 && fail20 {
			return i
		}
	}
	return -1
}

func findGap(tblName string, interval int, dqs []mydb.DaliyQuote) (DQPlot, error) {
	const GAP_MUL float64 = 1.015

	day := BASE_QDS_NR
	totalLen := interval + BASE_QDS_NR
	if len(dqs) != totalLen {
		fmt.Printf("ERR: %s day count is %d it should be %d\n", tblName, len(dqs), totalLen)
		return DQPlot{}, ErrTooFewDays
	}
	if dqs[totalLen-1].Volume < 300 {
		return DQPlot{}, ErrNotInsterested
	}

	findings := 0
	foundDay := -1
	candle := []DQCandle{}
	hline := []float64{}

	candle = append(candle, genCandle(dqs[day]))
	day += 1
	for day < totalLen {
		dq1 := dqs[day-1] // former day
		dq2 := dqs[day]   // current loop day

		h1 := math.Max(dq1.Open, dq1.Close)
		h2 := math.Max(dq2.Open, dq2.Close)

		l1 := math.Min(dq1.Open, dq1.Close)
		l2 := math.Min(dq2.Open, dq2.Close)

		highGap := l2 - h1
		lowGap := l1 - h2

		if findings != TYPE_GAP_CALL && highGap > 0 {
			if h1*GAP_MUL < l2 {
				foundDay = day
				findings = TYPE_GAP_CALL
				hline = []float64{float64(foundDay - 1 - BASE_QDS_NR), h1, float64(foundDay - BASE_QDS_NR), l2}
			}
		}
		if findings != TYPE_GAP_PUT && lowGap > 0 {
			if h2*GAP_MUL < l1 {
				foundDay = day
				findings = TYPE_GAP_PUT
				hline = []float64{float64(foundDay - 1 - BASE_QDS_NR), l1, float64(foundDay - BASE_QDS_NR), h2}
			}
		}

		if tblName == "stk3041" {
			fmt.Printf("fd=%d h=%f,%f l=%f,%f finding=%s\n", foundDay-BASE_QDS_NR, h1, h2, l1, l2, TypeToStr(findings))
		}

		candle = append(candle, genCandle(dq2))
		day += 1
	}

	if findings == 0 {
		return DQPlot{}, ErrNotInsterested
	}

	ma5 := genMA(dqs, 5)
	ma10 := genMA(dqs, 10)
	ma20 := genMA(dqs, 20)

	if findings == TYPE_GAP_CALL {
		closeDay := findGapCallClose(interval, foundDay, dqs, ma5, ma10, ma20)
		if closeDay == -1 && foundDay > BASE_QDS_NR+3 {
			return DQPlot{}, ErrNotInsterested
		}
		lastDays := totalLen - closeDay
		if lastDays < 3 {
			findings = TYPE_GAP_CALL_CLOSED
		} else {
			return DQPlot{}, ErrNotInsterested
		}
	}
	if findings == TYPE_GAP_PUT {
		closeDay := findGapPutClose(interval, foundDay, dqs, ma5, ma10, ma20)
		if closeDay == -1 && foundDay > BASE_QDS_NR+3 {
			return DQPlot{}, ErrNotInsterested
		}
		lastDays := totalLen - closeDay
		if lastDays < 3 {
			findings = TYPE_GAP_PUT_CLOSED
		} else {
			return DQPlot{}, ErrNotInsterested
		}
	}

	code, _ := strings.CutPrefix(tblName, "stk")
	plot := genPlot(code, findings, candle, ma5, ma10, ma20, hline)
	return plot, nil
}
