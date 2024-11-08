package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	mydb "myDatabase"
)

const TWSE_URL_FMT string = "https://www.twse.com.tw/rwd/en/afterTrading/MI_INDEX?date=%04d%02d%02d&type=ALLBUT0999"

type Payload struct {
	Groups []json.RawMessage `json:"groups"`
	Tables []json.RawMessage `json:"tables"`
	Params []json.RawMessage `json:"params"`
	Stat   []json.RawMessage `json:"stat"`
	Date   []json.RawMessage `json:"date"`
}
type Tables struct {
	Title  string            `json:"title"`
	Fields []string          `json:"fields"`
	Data   [][]string        `json:"data"`
	Groups []json.RawMessage `json:"groups"`
	Notes  []string          `json:"notes"`
}

func fetchTWSE(y int, m int, d int) error {
	twseURL := fmt.Sprintf(TWSE_URL_FMT, y, m, d)
	resp, err := http.Get(twseURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error: received status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var payload Payload
	err = json.Unmarshal(body, &payload)
	if err != nil {
		return err
	}

	var dqmap Tables
	found := false
	for _, ent := range payload.Tables {
		err := json.Unmarshal(ent, &dqmap)
		if err != nil {
			log.Fatalf("Failed to unmarshal second table: %v", err)
		}
		if strings.Contains(dqmap.Title, "Daily Quotes") {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("Daily Quotes Not found")
	}
	saveDailyQuotes(dqmap.Data, y, m, d)
	return nil
}

func saveDailyQuotes(dqarr [][]string, y int, m int, d int) error {
	for _, ent := range dqarr {
		code, dq, err := formatToDailyQuote(ent, y, m, d)
		if err != nil {
			return err
		}
		err = mydb.AddDailyQuote(code, dq)
		if err != nil {
			return err
		}
	}
	return nil
}

func formatToDailyQuote(entry []string, y int, m int, d int) (string, mydb.DaliyQuote, error) {
	dq := mydb.DaliyQuote{Year: y, Month: m, Day: d}

	if len(entry) < 15 {
		return "", dq, fmt.Errorf("Wrong length of entry: %d", len(entry))
	}

	code := entry[0]

	v, err := strconv.ParseInt(strings.Replace(entry[1], ",", "", -1), 10, 32)
	if err != nil {
		return code, dq, err
	}
	trans, err := strconv.ParseInt(strings.Replace(entry[2], ",", "", -1), 10, 32)
	if err != nil {
		return code, dq, err
	}
	tval, err := strconv.ParseInt(strings.Replace(entry[3], ",", "", -1), 10, 32)
	if err != nil {
		return code, dq, err
	}
	open, err := strconv.ParseFloat(strings.Replace(entry[4], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}
	high, err := strconv.ParseFloat(strings.Replace(entry[5], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}
	low, err := strconv.ParseFloat(strings.Replace(entry[6], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}
	close, err := strconv.ParseFloat(strings.Replace(entry[7], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}
	lbp, err := strconv.ParseFloat(strings.Replace(entry[10], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}
	lbv, err := strconv.ParseInt(strings.Replace(entry[11], ",", "", -1), 10, 32)
	if err != nil {
		return code, dq, err
	}
	lap, err := strconv.ParseFloat(strings.Replace(entry[12], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}
	lav, err := strconv.ParseInt(strings.Replace(entry[13], ",", "", -1), 10, 32)
	if err != nil {
		return code, dq, err
	}
	pe, err := strconv.ParseFloat(strings.Replace(entry[14], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}

	dq.Volume = v
	dq.Transaction = int(trans)
	dq.Value = tval
	dq.Open = open
	dq.Close = close
	dq.High = high
	dq.Low = low
	dq.LastBidPrice = lbp
	dq.LastBidVol = int(lbv)
	dq.LastAskPrice = lap
	dq.LastAskVol = int(lav)
	dq.PE = pe

	return code, dq, nil
}
