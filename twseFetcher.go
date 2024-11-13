package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	mydb "myDatabase"
)

const TWSE_URL_FMT string = "https://www.twse.com.tw/rwd/en/afterTrading/MI_INDEX?date=%04d%02d%02d&type=ALLBUT0999"

var ErrNoEntry error = errors.New("no data")
var ErrFetchBad error = errors.New("fetch bad status code")
var ErrNoVolume error = errors.New("no trade volume")
var ErrIsETF error = errors.New("excluded ETF")

var lastFetchTime time.Time = time.Now()

type Payload struct {
	Groups []json.RawMessage `json:"groups"`
	Tables []json.RawMessage `json:"tables"`
	// Params []json.RawMessage `json:"params"`
	// Stat   string            `json:"stat"`
	// Date   string            `json:"date"`
}
type Tables struct {
	Title  string            `json:"title"`
	Fields []string          `json:"fields"`
	Data   [][]string        `json:"data"`
	Groups []json.RawMessage `json:"groups"`
	Notes  []string          `json:"notes"`
}

func needFetch(now time.Time, end time.Time) bool {
	ny, nm, nd := now.Date()
	ey, em, ed := end.Date()

	if ny > ey || ny == ey && nm > em || ny == ey && nm == em && nd > ed {
		return true
	}
	return false
}

func updateFetch(interval int) error {
	var i int = 0
	now := time.Now()
	if now.Hour() < 15 {
		now = now.AddDate(0, 0, -1)
	}
	updateNow := now

	start, end := mydb.GetDQCheckedDate()
	fmt.Printf("Check date is %s->%s (%s)\n", start.Format("20060102"), end.Format("20060102"), now.Format("20060102"))

	for needFetch(now, end) && i < interval {
		if isWeekend(now) {
			now = now.AddDate(0, 0, -1)
			continue
		}
		y, m, d := now.Date()
		err := fetchTWSE(y, int(m), d)
		if err == ErrNoEntry {
			now = now.AddDate(0, 0, -1)
			continue
		}
		if err != nil {
			return err
		}
		i = i + 1
		now = now.AddDate(0, 0, -1)
	}

	mydb.SetDQCheckedDate(now, updateNow)
	return nil
}

func fetchTWSE(y int, m int, d int) error {
	duration := time.Since(lastFetchTime)
	if duration.Seconds() < 2 {
		time.Sleep(2 * time.Second)
	}

	fmt.Printf("Fetching TWSE %d/%d/%d...\n", y, m, d)
	lastFetchTime = time.Now()
	twseURL := fmt.Sprintf(TWSE_URL_FMT, y, m, d)
	resp, err := http.Get(twseURL)
	if err != nil {
		fmt.Printf("http get failed = %s\n", err.Error())
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Bad status = %d\n", resp.StatusCode)
		return ErrFetchBad
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("ReadAll error = %s\n", err.Error())
		return err
	}

	var payload Payload
	err = json.Unmarshal(body, &payload)
	if err != nil {
		fmt.Printf("Unmarshal payload error = %s\n", err.Error())
		return err
	}

	if payload.Tables == nil {
		fmt.Printf("No entry\n")
		return ErrNoEntry
	}

	var dqmap Tables
	found := false
	for _, ent := range payload.Tables {
		err := json.Unmarshal(ent, &dqmap)
		if err != nil {
			fmt.Printf("Unmarshal dqmap error = %s\n", err.Error())
			return err
		}
		if strings.Contains(dqmap.Title, "Daily Quotes") {
			found = true
			break
		}
	}

	if !found {
		fmt.Printf("No Daily Quites\n")
		return ErrNoEntry
	}

	return saveDailyQuotes(dqmap.Data, y, m, d)
}

func saveDailyQuotes(dqarr [][]string, y int, m int, d int) error {
	for _, ent := range dqarr {
		code, dq, err := formatToDailyQuote(ent, y, m, d)
		if err == ErrIsETF {
			continue
		}
		if err != nil {
			return err
		}
		if dq.Volume == 0 {
			dqOld, err := mydb.FindPrevDailyQuote(code, y, m, d)
			if err != nil {
				return err
			}
			dq.Open = dqOld.Close
			dq.High = dqOld.Close
			dq.Low = dqOld.Close
			dq.Close = dqOld.Close
			dq.PE = dqOld.PE
		}
		err = mydb.AddDailyQuote(code, dq)
		if err != nil {
			return err
		}
	}
	fmt.Println("Save DQ Complete")
	return nil
}

func formatToDailyQuote(entry []string, y int, m int, d int) (string, mydb.DaliyQuote, error) {
	var open, high, low, close, pe float64
	var trans, tval int64
	dq := mydb.DaliyQuote{Year: y, Month: m, Day: d}

	if len(entry) < 15 {
		return "", dq, fmt.Errorf("wrong length of entry: %d", len(entry))
	}

	code := entry[0]
	if strings.HasPrefix(entry[0], "0") {
		return code, dq, ErrIsETF
	}

	fmt.Printf("Formating %s... %v#\r", code, entry)

	v, err := strconv.ParseInt(strings.Replace(entry[1], ",", "", -1), 10, 64)
	if err != nil {
		return code, dq, err
	}

	if v == 0 || entry[4] == "--" || entry[5] == "--" || entry[6] == "--" || entry[7] == "--" {
		v = 0
		goto skip
	}

	trans, err = strconv.ParseInt(strings.Replace(entry[2], ",", "", -1), 10, 32)
	if err != nil {
		return code, dq, err
	}
	tval, err = strconv.ParseInt(strings.Replace(entry[3], ",", "", -1), 10, 64)
	if err != nil {
		return code, dq, err
	}
	open, err = strconv.ParseFloat(strings.Replace(entry[4], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}
	high, err = strconv.ParseFloat(strings.Replace(entry[5], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}
	low, err = strconv.ParseFloat(strings.Replace(entry[6], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}
	close, err = strconv.ParseFloat(strings.Replace(entry[7], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}
	pe, err = strconv.ParseFloat(strings.Replace(entry[14], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}

skip:
	if v == 0 {
		open = 0
		high = 0
		low = 0
		close = 0
		pe = 0
	}

	dq.Volume = v
	dq.Trans = int(trans)
	dq.Value = tval
	dq.Open = open
	dq.Close = close
	dq.High = high
	dq.Low = low
	dq.PE = pe

	return code, dq, nil
}
