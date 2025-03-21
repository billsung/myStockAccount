package main

import (
	"fmt"
	"strings"

	mydb "myDatabase"
)

func findGapCallClose(foundDay int, dqs []mydb.DaliyQuote, ma5 []DataPoint, ma10 []DataPoint, ma20 []DataPoint) int {
	base := dqs[foundDay-1].High
	for i := foundDay + 1; i < len(dqs); i = i + 1 {
		maDay := i - BASE_QDS_NR
		avg5 := ma5[maDay].Y
		avg10 := ma10[maDay].Y
		avg20 := ma20[maDay].Y

		if dqs[i].Close < base {
			return i
		}

		fail5 := dqs[i].Close <= avg5
		fail10 := dqs[i].Close <= avg10
		fail20 := dqs[i].Close <= avg20

		if fail5 && fail10 && fail20 {
			return i
		}
	}
	return -1
}

func findGapPutClose(foundDay int, dqs []mydb.DaliyQuote, ma5 []DataPoint, ma10 []DataPoint, ma20 []DataPoint) int {
	base := dqs[foundDay-1].Low
	for i := foundDay + 1; i < len(dqs); i = i + 1 {
		maDay := i - BASE_QDS_NR
		avg5 := ma5[maDay].Y
		avg10 := ma10[maDay].Y
		avg20 := ma20[maDay].Y

		if dqs[i].Close > base {
			// fmt.Printf("close=%f base=%f\n", dqs[i].Close, base)
			return i
		}

		fail5 := dqs[i].Close >= avg5
		fail10 := dqs[i].Close >= avg10
		fail20 := dqs[i].Close >= avg20

		if fail5 && fail10 && fail20 {
			// fmt.Printf("close=%f ma=%f,%f,%f\n", dqs[i].Close, avg5, avg10, avg20)
			return i
		}
	}
	return -1
}

func genLineDP(dq *mydb.DaliyQuote, dqEnd *mydb.DaliyQuote, y float64) []DataPoint {
	a := DataPoint{X: toMMDD(dq), Y: y}
	b := DataPoint{X: toMMDD(dqEnd), Y: y}
	return []DataPoint{a, b}
}

func findGap(option string, tblName string, interval int, dqs []mydb.DaliyQuote) (Result, error) {
	const FIND_TYP_ALL = 0
	const FIND_TYP_CALL = 1
	const FIND_TYP_PUT = 2
	findType := 0
	day := BASE_QDS_NR
	totalLen := len(dqs)
	if len(dqs) < BASE_QDS_NR+interval {
		fmt.Printf("ERR: %s day count is %d. It should over %d\n", tblName, len(dqs), BASE_QDS_NR+interval)
		return Result{}, ErrTooFewDays
	}
	skipDay := totalLen - interval/2
	if dqs[totalLen-1].Volume < MIN_INTRESTED_VOL {
		return Result{}, ErrNotInsterested
	}
	if option == "Call" {
		findType = FIND_TYP_CALL
	} else if option == "Put" {
		findType = FIND_TYP_PUT
	} else {
		findType = FIND_TYP_ALL
	}

	findings := 0
	foundDay := -1
	foundGap := 0.0
	labels := []string{}
	vols := []DataPoint{}
	candle := []DataPoint{}
	hline1 := Dataset{}
	hline2 := Dataset{}

	for day < skipDay {
		mmdd := fmt.Sprintf("%02d%02d", dqs[day].Month, dqs[day].Day)
		candle = append(candle, toCandleDataPoint(mmdd, &dqs[day]))
		vols = append(vols, toVolumeDataPoint(mmdd, &dqs[day]))
		labels = append(labels, toMMDD(&dqs[day]))
		day += 1
	}

	for day < totalLen {
		dq1 := dqs[day-1] // former day
		dq2 := dqs[day]   // current loop day

		h1 := dq1.High
		h2 := dq2.High

		l1 := dq1.Low
		l2 := dq2.Low

		highGap := l2 - h1
		lowGap := l1 - h2

		if highGap > 0 {
			if findings != TYPE_GAP_CALL || highGap > foundGap {
				foundDay = day
				findings = TYPE_GAP_CALL
				foundGap = highGap

				hline1 = GenLineDataset("upper", genLineDP(&dq2, &dqs[totalLen-1], l2), "#eb4034")
				hline2 = GenLineDataset("downer", genLineDP(&dq1, &dqs[totalLen-1], h1), "#f5405e")
			}
		}
		if lowGap > 0 {
			if findings != TYPE_GAP_PUT || lowGap > foundGap {
				foundDay = day
				findings = TYPE_GAP_PUT
				foundGap = lowGap
				hline1 = GenLineDataset("upper", genLineDP(&dq1, &dqs[totalLen-1], l1), "#007a3d")
				hline2 = GenLineDataset("downer", genLineDP(&dq2, &dqs[totalLen-1], h2), "#11bf4b")
			}
		}

		// if tblName == "stk1618" {
		// 	fmt.Printf("(%d)fd=%d h=%f,%f l=%f,%f hg=%f lg=%f finding=%s\n",
		// 		day, foundDay, h1, h2, l1, l2, highGap, lowGap, TypeToStr(findings))
		// }

		mmdd := toMMDD(&dq2)
		candle = append(candle, toCandleDataPoint(mmdd, &dq2))
		vols = append(vols, toVolumeDataPoint(mmdd, &dq2))
		labels = append(labels, mmdd)
		day += 1
	}

	if findings == 0 {
		return Result{}, ErrNotInsterested
	}

	ma5 := genMA(dqs, 5)
	ma10 := genMA(dqs, 10)
	ma20 := genMA(dqs, 20)

	if findings == TYPE_GAP_CALL {
		closeDay := findGapCallClose(foundDay, dqs, ma5, ma10, ma20)
		if closeDay == -1 && foundDay < (totalLen-4) {
			return Result{}, ErrNotInsterested
		} else if closeDay != -1 {
			lastDays := totalLen - closeDay
			if lastDays < 2 {
				findings = TYPE_GAP_CALL_CLOSED
			} else {
				return Result{}, ErrNotInsterested
			}
		}
	}
	if findings == TYPE_GAP_PUT {
		closeDay := findGapPutClose(foundDay, dqs, ma5, ma10, ma20)
		// if tblName == "stk1618" {
		// 	fmt.Printf("closeDay=%d\n", closeDay)
		// }

		if closeDay == -1 && foundDay < (totalLen-4) {
			return Result{}, ErrNotInsterested
		} else if closeDay != -1 {
			lastDays := totalLen - closeDay
			if lastDays < 2 {
				findings = TYPE_GAP_PUT_CLOSED
			} else {
				return Result{}, ErrNotInsterested
			}
		}
	}

	if findType == FIND_TYP_CALL {
		if findings == TYPE_GAP_PUT || findings == TYPE_GAP_CALL_CLOSED {
			return Result{}, ErrNotInsterested
		}

	}
	if findType == FIND_TYP_PUT {
		if findings == TYPE_GAP_CALL || findings == TYPE_GAP_PUT_CLOSED {
			return Result{}, ErrNotInsterested
		}
	}

	code, _ := strings.CutPrefix(tblName, "stk")
	dataset := []Dataset{GenCandleDataset(code, candle)}
	dataset = append(dataset, GenVolumeDataset(vols))
	dataset = append(dataset, GenLineDataset("ma5", ma5, MA5_SKYBLUE))
	dataset = append(dataset, GenLineDataset("ma10", ma10, MA10_YELLOW))
	dataset = append(dataset, GenLineDataset("ma20", ma20, MA20_PURPLE))

	dataset = append(dataset, hline1)
	dataset = append(dataset, hline2)

	config := GenCandleStickChartConfig(labels, dataset)
	return Result{Config: config, Info: TypeToStr(findings)}, nil
}
