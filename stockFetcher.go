package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	mydb "myDatabase"
)

const FETCH_DELAY_SEC float64 = 3

const TWSE_API_URL string = "https://www.twse.com.tw/exchangeReport/MI_INDEX?response=json&date=%04d%02d%02d&type=ALLBUT0999"
const TPEX_API_URL string = "https://www.tpex.org.tw/www/zh-tw/afterTrading/otc?date=%04d/%02d/%02d&type=EW&response=json"

const DATA_TYPE_TWSE int = 1
const DATA_TYPE_TPEX int = 2

var ErrNoEntry error = errors.New("no data")
var ErrFetchBad error = errors.New("fetch bad status code")
var ErrNoVolume error = errors.New("no trade volume")
var ErrNotStock error = errors.New("not ordinary stock")

var lastTWSEFetchTime time.Time = time.Now()
var lastTPExFetchTime time.Time = time.Now()

type TWSEReport struct {
	Tables []TWSETable `json:"tables"`
	Stat   string      `json:"stat"`
	Date   string      `json:"date"`
}
type TWSETable struct {
	Title  string            `json:"title"`
	Fields []json.RawMessage `json:"fields"`
	Data   [][]string        `json:"data"`
	Hints  string            `json:"hints"`
}
type TPExReport struct {
	Date      string       `json:"date"`
	Tables    []TPExTables `json:"tables"`
	FlagField string       `json:"flagField"`
	Stat      string       `json:"stat"`
}
type TPExTables struct {
	Title              string            `json:"title"`
	Subtitle           string            `json:"subtitle"`
	Date               string            `json:"date,omitempty"`
	ListedCompanies    string            `json:"listedCompanies,omitempty"`
	TotalTradingAmount string            `json:"totalTradingAmount,omitempty"`
	TotalTradingShares string            `json:"totalTradingShares,omitempty"`
	TotalTranscations  string            `json:"totalTranscations,omitempty"`
	TotalCount         int               `json:"totalCount"`
	Fields             []string          `json:"fields"`
	Data               [][]string        `json:"data"`
	Notes              []json.RawMessage `json:"notes"`
}

func dayBeforeInclude(D1 time.Time, D2 time.Time) bool {
	y1, m1, d1 := D1.Date()
	y2, m2, d2 := D2.Date()

	if y1 < y2 || y1 == y2 && m1 < m2 || y1 == y2 && m1 == m2 && d1 <= d2 {
		return true
	}
	return false
}

func updateFetch() error {
	var i int = 0
	end := time.Now()
	if end.Hour() < 15 {
		end = end.AddDate(0, 0, -1)
	}

	start := mydb.GetDQCheckedDate()
	now := start.AddDate(0, 0, 1)
	fmt.Printf("Check date is %s->%s (%s)\n", start.Format("20060102"), end.Format("20060102"), now.Format("20060102"))

	for dayBeforeInclude(now, end) {
		if isWeekend(now) {
			now = now.AddDate(0, 0, 1)
			continue
		}
		y, m, d := now.Date()
		err := fetch(DATA_TYPE_TWSE, y, int(m), d)
		if err != nil {
			return err
		}
		err = fetch(DATA_TYPE_TPEX, y, int(m), d)
		if err != nil {
			return err
		}
		i = i + 1
		now = now.AddDate(0, 0, 1)
	}

	mydb.SetDQCheckedDate(end)
	return nil
}

func fetch(stkType int, y int, m int, d int) error {
	var url string
	var duration time.Duration
	if stkType == DATA_TYPE_TWSE {
		url = TWSE_API_URL
		duration = time.Since(lastTWSEFetchTime)
	} else if stkType == DATA_TYPE_TPEX {
		duration = time.Since(lastTPExFetchTime)
		url = TPEX_API_URL
	} else {
		return errors.New("invalid stock type")
	}

	if duration.Seconds() < FETCH_DELAY_SEC {
		time.Sleep(time.Duration(FETCH_DELAY_SEC) * time.Second)
	}

	url = fmt.Sprintf(url, y, m, d)
	fmt.Printf("Fetching %s (%d/%d/%d)...\n", url, y, m, d)
	if stkType == DATA_TYPE_TWSE {
		lastTWSEFetchTime = time.Now()
	} else if stkType == DATA_TYPE_TPEX {
		lastTPExFetchTime = time.Now()
	}

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("http get failed = %s\n", err.Error())
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Read error:", err)
		return err
	}

	switch stkType {
	case DATA_TYPE_TWSE:
		var report TWSEReport
		err = json.Unmarshal(body, &report)
		if err != nil {
			return err
		}
		if report.Tables == nil {
			return ErrNoEntry
		}
		for _, ent := range report.Tables {
			if !strings.Contains(ent.Title, "每日收盤行情") {
				continue
			}
			for _, data := range ent.Data {
				err = saveDailyQuote(DATA_TYPE_TWSE, data, y, m, d)
				if err != nil {
					break
				}
			}
		}
	case DATA_TYPE_TPEX:
		var report TPExReport
		err = json.Unmarshal(body, &report)
		if err != nil {
			return err
		}
		if report.Tables[0].TotalCount == 0 {
			return ErrNoEntry
		}
		for _, ent := range report.Tables {
			if !strings.Contains(ent.Title, "上櫃股票每日收盤行情") {
				continue
			}
			for _, data := range ent.Data {
				err = saveDailyQuote(DATA_TYPE_TPEX, data, y, m, d)
				if err != nil {
					break
				}
			}
		}
	}

	if err != nil {
		fmt.Printf("Unmarshal data type=%d error = %s\n", stkType, err.Error())
		return err
	}

	fmt.Printf("Save DQ %d/%d/%d for type%d Complete\n", y, m, d, stkType)
	return err
}

func saveDailyQuote(stkType int, data []string, y int, m int, d int) error {
	var code string
	var dq mydb.DaliyQuote
	var err error

	switch stkType {
	case DATA_TYPE_TWSE:
		code, dq, err = formatTWSE(data, y, m, d)
	case DATA_TYPE_TPEX:
		code, dq, err = formatTPEx(data, y, m, d)
	default:
		return errors.New("invalid stock type")
	}

	if err == ErrNotStock {
		return nil
	} else if err != nil {
		log.Fatalf("Error: %s\n", err.Error())
		return err
	}

	fmt.Printf("Processing %s...\r", code)
	existDQArr, err := mydb.GetDailyQuote(mydb.STKPREFIX+code, 1)
	if err != nil && err != mydb.ErrNoSuchTable {
		fmt.Printf("Error: %s\n", err.Error())
		return err
	}

	if err != mydb.ErrNoSuchTable {
		existDQ := existDQArr[0]
		if existDQ.Day == d && existDQ.Month == m && existDQ.Year == y {
			// Already exist.
			return nil
		}

		if dq.Volume == 0 {
			dq.Open = existDQ.Close
			dq.High = existDQ.Close
			dq.Low = existDQ.Close
			dq.Close = existDQ.Close
		}
	}

	err = mydb.AddDailyQuote(code, &dq)
	if err != nil {
		log.Fatalf("Error: %s\n", err.Error())
		return err
	}
	return err
}

func formatTWSE(data []string, y int, m int, d int) (string, mydb.DaliyQuote, error) {
	var open, high, low, close float64
	var volume, trans, tval int64
	var err error

	const (
		IDX_CODE         = 0
		IDX_NAME         = 1
		IDX_VOLUME       = 2
		IDX_TRANS_NR     = 3
		IDX_TARANS_VALUE = 4
		IDX_OPEN         = 5
		IDX_HIGH         = 6
		IDX_LOW          = 7
		IDX_CLOSE        = 8
		IDX_DIRECTION    = 9
		IDX_PRICE_DIFF   = 10
		IDX_LAST_ASK     = 11
		IDX_LAST_ASK_VOL = 12
		IDX_LAST_BID     = 13
		IDX_LAST_BID_VOL = 14
		IDX_PE           = 15
	)

	dq := mydb.DaliyQuote{Year: y, Month: m, Day: d}

	code := data[IDX_CODE]
	// fmt.Printf("Formating %s...\n", code)

	if isWarrant(code) {
		return code, dq, ErrNotStock
	}

	if strings.Contains(data[IDX_CLOSE], "--") {
		volume = 0
		trans = 0
		tval = 0
		open = 0
		high = 0
		low = 0
		close = 0
		goto finish
	}

	volume, err = strconv.ParseInt(strings.Replace(data[IDX_VOLUME], ",", "", -1), 10, 64)
	if err != nil {
		return code, dq, err
	}

	trans, err = strconv.ParseInt(strings.Replace(data[IDX_TRANS_NR], ",", "", -1), 10, 64)
	if err != nil {
		return code, dq, err
	}
	tval, err = strconv.ParseInt(strings.Replace(data[IDX_TARANS_VALUE], ",", "", -1), 10, 64)
	if err != nil {
		return code, dq, err
	}
	open, err = strconv.ParseFloat(strings.Replace(data[IDX_OPEN], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}
	high, err = strconv.ParseFloat(strings.Replace(data[IDX_HIGH], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}
	low, err = strconv.ParseFloat(strings.Replace(data[IDX_LOW], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}
	close, err = strconv.ParseFloat(strings.Replace(data[IDX_CLOSE], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}

	_, err = mydb.RefLookupNameByCode(code)
	if err == nil {
		goto finish
	}
	err = mydb.AddRef(code, data[IDX_NAME])
	if err != nil {
		fmt.Printf("Failed to Add Ref! for %s -> %s\n", code, data[IDX_NAME])
	}

finish:
	dq.Volume = volume
	dq.Trans = int(trans)
	dq.Value = tval
	dq.Open = open
	dq.Close = close
	dq.High = high
	dq.Low = low
	return code, dq, nil
}

func formatTPEx(data []string, y int, m int, d int) (string, mydb.DaliyQuote, error) {
	var open, high, low, close float64
	var volume, trans, tval int64
	var err error

	const (
		IDX_CODE           = 0
		IDX_NAME           = 1
		IDX_CLOSE          = 2
		IDX_PRICE_DIFF     = 3
		IDX_OPEN           = 4
		IDX_HIGH           = 5
		IDX_LOW            = 6
		IDX_VOLUME         = 7
		IDX_TARANS_VALUE   = 8
		IDX_TRANS_NR       = 9
		IDX_LAST_ASK       = 10
		IDX_LAST_ASK_VOL1K = 11
		IDX_LAST_BID       = 12
		IDX_LAST_BID_VOL1K = 13
		IDX_STOCK_QTY      = 14
		IDX_NXT_D_HIGHEST  = 15
		IDX_NXT_D_LOWEST   = 16
	)

	dq := mydb.DaliyQuote{Year: y, Month: m, Day: d}

	code := data[IDX_CODE]
	// fmt.Printf("Formating %s...\n", code)

	if isWarrant(code) {
		return code, dq, ErrNotStock
	}

	if strings.Contains(data[IDX_CLOSE], "--") {
		volume = 0
		trans = 0
		tval = 0
		open = 0
		high = 0
		low = 0
		close = 0
		goto finish
	}

	volume, err = strconv.ParseInt(strings.Replace(data[IDX_VOLUME], ",", "", -1), 10, 64)
	if err != nil {
		return code, dq, err
	}

	trans, err = strconv.ParseInt(strings.Replace(data[IDX_TRANS_NR], ",", "", -1), 10, 64)
	if err != nil {
		return code, dq, err
	}
	tval, err = strconv.ParseInt(strings.Replace(data[IDX_TARANS_VALUE], ",", "", -1), 10, 64)
	if err != nil {
		return code, dq, err
	}
	open, err = strconv.ParseFloat(strings.Replace(data[IDX_OPEN], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}
	high, err = strconv.ParseFloat(strings.Replace(data[IDX_HIGH], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}
	low, err = strconv.ParseFloat(strings.Replace(data[IDX_LOW], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}
	close, err = strconv.ParseFloat(strings.Replace(data[IDX_CLOSE], ",", "", -1), 64)
	if err != nil {
		return code, dq, err
	}

	_, err = mydb.RefLookupNameByCode(code)
	if err == nil {
		goto finish
	}
	err = mydb.AddRef(code, data[IDX_NAME])
	if err != nil {
		fmt.Printf("Failed to Add Ref! for %s -> %s\n", code, data[IDX_NAME])
	}

finish:
	if volume == 0 {
		open = 0
		high = 0
		low = 0
		close = 0
	}

	dq.Volume = volume
	dq.Trans = int(trans)
	dq.Value = tval
	dq.Open = open
	dq.Close = close
	dq.High = high
	dq.Low = low
	return code, dq, nil
}

func isWarrant(code string) bool {
	var rules = []string{
		"^02[0-9][0-9][0-9][0-9]$", // ETN
		"^02[0-9][0-9][0-9][LRB]$",
		// 槓桿ETN
		// 反向ETN
		// 債券ETN
		"^0[3-8][0-9][0-9][0-9][0-9]$", // 上市國內標的認購權證
		"^0[3-8][0-9][0-9][0-9][PFQCBXY]$",
		// 上市國內標的認售權證
		// 上市外國標的認購權證
		// 上市外國標的認售權證
		// 上市國內標的下限型認購權證
		// 上市國內標的上限型認售權證
		// 上市國內標的可展延下限型認購權證
		// 上市國內標的可展延上限型認售權證
		"^7[0-3][0-9][0-9][0-9][0-9]$", // 上櫃國內標的認購權證
		"^7[0-3][0-9][0-9][0-9][PFQCBXY]$",
		// 上櫃國內標的認售權證
		// 上櫃外國標的認購權證
		// 上櫃外國標的認售權證
		// 上櫃國內標的下限型認購權證
		// 上櫃國內標的上限型認售權證
		// 上櫃國內標的可展延下限型認購權證
		// 上櫃國內標的可展延上限型認售權證
	}

	for _, rule := range rules {
		matched, err := regexp.MatchString(rule, code)
		if err != nil {
			log.Fatal("Regix error")
		}
		if matched {
			return true
		}
	}
	return false
}
