package main

import (
	"fmt"
	"strings"

	mydb "myDatabase"
)

func toHumanized(avg int64) string {
	index := 0
	for avg >= 100000 {
		index += 1
		avg /= 1000
	}

	unit := ""
	switch index {
	case 0:
		break
	case 1:
		unit = "K"
	case 2:
		unit = "M"
	case 3:
		unit = "G"
	case 4:
		unit = "T"
	default:
		unit = "?"
	}

	return fmt.Sprintf("%d%s", avg, unit)
}

func findVolBurst(tblName string, interval int, dqs []mydb.DaliyQuote) (ScanResult, error) {
	const BURST_MUL int64 = 4

	totalLen := interval + BASE_QDS_NR
	if len(dqs) != totalLen {
		fmt.Printf("ERR: %s day count is %d it should be %d\n", tblName, len(dqs), totalLen)
		return ScanResult{}, ErrTooFewDays
	}
	if dqs[totalLen-1].Volume < 300 {
		return ScanResult{}, ErrNotInsterested
	}

	if dqs[totalLen-1].Close < dqs[totalLen-1].Open {
		diff := dqs[totalLen-1].Open - dqs[totalLen-1].Close
		bottomShadowLen := dqs[totalLen-1].Low - dqs[totalLen-1].Close
		if bottomShadowLen < diff*2 {
			return ScanResult{}, ErrNotInsterested
		}
	}

	candles := []DataPoint{}
	volumes := []DataPoint{}
	labels := []string{}
	var avg int64 = 0
	for i := BASE_QDS_NR; i < totalLen-1; i += 1 {
		avg += (dqs[i].Volume * int64(i-BASE_QDS_NR-1))
		mmdd := toMMDD(&dqs[i])
		candles = append(candles, toCandleDataPoint(mmdd, &dqs[i]))
		volumes = append(volumes, toVolumeDataPoint(mmdd, &dqs[i]))
		labels = append(labels, mmdd)
	}
	avg /= int64((1 + interval - 1) * (interval - 1) / 2)
	mmdd := toMMDD(&dqs[totalLen-1])
	candles = append(candles, toCandleDataPoint(mmdd, &dqs[totalLen-1]))
	volumes = append(volumes, toVolumeDataPoint(mmdd, &dqs[totalLen-1]))
	labels = append(labels, mmdd)

	if avg*BURST_MUL >= dqs[totalLen-1].Volume {
		return ScanResult{}, ErrNotInsterested
	}

	ma5 := genMA(dqs, 5)
	ma10 := genMA(dqs, 10)
	ma20 := genMA(dqs, 20)

	code, _ := strings.CutPrefix(tblName, "stk")
	dataset := []Dataset{GenCandleDataset(code, candles)}
	dataset = append(dataset, GenVolumeDataset(volumes))
	dataset = append(dataset, GenLineDataset("ma5", ma5, MA5_SKYBLUE))
	dataset = append(dataset, GenLineDataset("ma10", ma10, MA10_YELLOW))
	dataset = append(dataset, GenLineDataset("ma20", ma20, MA20_PURPLE))

	config := GenChartConfig(labels, dataset)
	info := fmt.Sprintf("avg=%s(x%.2f)", toHumanized(avg), float64(dqs[totalLen-1].Volume)/float64(avg))
	return ScanResult{Config: config, Info: info}, nil
}