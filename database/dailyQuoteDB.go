package myDatabase

import (
	"log"
)

const STKPREFIX string = "stk"

type DaliyQuote struct {
	Year         int     `json:"year"`
	Month        int     `json:"month"`
	Day          int     `json:"day"`
	Volume       int64   `json:"volume"`      // 成交股數
	Transaction  int     `json:"transaction"` // 交易筆數
	Value        int64   `json:"value"`       // 交易金額
	Open         float64 `json:"open"`
	High         float64 `json:"high"`
	Low          float64 `json:"low"`
	Close        float64 `json:"close"`
	LastBidPrice float64 `json:"last_bid_price"`
	LastBidVol   int     `json:"last_bid_vol"`
	LastAskPrice float64 `json:"last_ask_price"`
	LastAskVol   int     `json:"last_ask_vol"`
	PE           float64 `json:"pe"`
}

func checkStockTbl(code string) error {
	createTableSQL := `CREATE TABLE IF NOT EXISTS ` + STKPREFIX + code + ` (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		year INTEGER NOT NULL,
		month INTEGER NOT NULL,
		day INTEGER NOT NULL,
		volume INTEGER NOT NULL,
		transaction INTEGER NOT NULL,
		value INTEGER NOT NULL,
		open REAL NOT NULL,
		high REAL NOT NULL,
		low REAL NOT NULL,
		close REAL NOT NULL,
		last_bid_price REAL NOT NULL,
		last_bid_vol INTEGER NOT NULL,
		last_ask_price REAL NOT NULL,
		last_ask_vol INTEGER NOT NULL,
		pe REAL NOT NULL
	    );`

	if _, err := scanDB.Exec(createTableSQL); err != nil {
		log.Fatalf("Main: Failed to create table: %v", err)
		return err
	}
	return nil
}

func AddDailyQuote(code string, dq DaliyQuote) error {
	err := checkStockTbl(code)
	if err != nil {
		return err
	}

	tblName := STKPREFIX + code

	cmd := "INSERT INTO " + tblName +
		" (year, month, day, volume, transaction, value, open, high, low, close, last_bid_price, last_bid_vol, last_ask_price, last_ask_vol, pe)" +
		" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"

	_, err = scanDB.Exec(cmd, dq.Year, dq.Month, dq.Day, dq.Volume, dq.Transaction, dq.Value, dq.Open, dq.High, dq.Low, dq.Close, dq.LastBidPrice, dq.LastBidVol, dq.LastAskPrice, dq.LastAskVol, dq.PE)
	return err
}
