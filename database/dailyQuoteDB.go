package myDatabase

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

const STKPREFIX string = "stk"
const CHECKED_DATE_TABLE string = "checkdate"

type DaliyQuote struct {
	Year   int     `json:"year"`
	Month  int     `json:"month"`
	Day    int     `json:"day"`
	Volume int64   `json:"volume"` // 成交股數
	Trans  int     `json:"trans"`  // 交易筆數
	Value  int64   `json:"value"`  // 交易金額
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	PE     float64 `json:"pe"`
}

func initStockTbl() {
	cmd := `CREATE TABLE IF NOT EXISTS ` + CHECKED_DATE_TABLE + ` (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		startdate TEXT NOT NULL,
		enddate TEXT NOT NULL
	    )`

	if _, err := scanDB.Exec(cmd); err != nil {
		log.Fatalf("DQ: Failed to create checked-table: %v", err)
	}
}

func checkStockTbl(code string) error {
	cmd := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s%s (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		year INTEGER NOT NULL,
		month INTEGER NOT NULL,
		day INTEGER NOT NULL,
		volume INTEGER NOT NULL,
		trans INTEGER NOT NULL,
		value INTEGER NOT NULL,
		open REAL NOT NULL,
		high REAL NOT NULL,
		low REAL NOT NULL,
		close REAL NOT NULL,
		pe REAL NOT NULL
	    )`, STKPREFIX, code)

	if _, err := scanDB.Exec(cmd); err != nil {
		log.Fatalf("DQ: Failed to create StockTbl: %v\n%s", err, cmd)
		return err
	}
	return nil
}

func GetTableList() []string {
	cmd := "SELECT name FROM sqlite_master WHERE type='table'"
	rows, err := scanDB.Query(cmd)
	if err != nil {
		return nil
	}
	defer rows.Close()

	tblList := []string{}
	for rows.Next() {
		var str string
		err := rows.Scan(&str)
		if err != nil {
			return nil
		}
		if strings.HasPrefix(str, "stk") {
			tblList = append(tblList, str)
		}
	}
	return tblList
}

func GetDQCheckedDate() (time.Time, time.Time) {
	var st, ed string
	var s, e time.Time
	cmd := fmt.Sprintf("SELECT startdate, enddate FROM " + CHECKED_DATE_TABLE)
	row := scanDB.QueryRow(cmd)
	err := row.Scan(&st, &ed)
	if err != nil {
		fmt.Println("GetChecked table failed.")
		goto errOut
	}

	s, err = time.Parse("20060102", st)
	if err != nil {
		fmt.Println("GetChecked parse start failed.")
		goto errOut
	}
	e, err = time.Parse("20060102", ed)
	if err != nil {
		fmt.Println("GetChecked parse end failed.")
		goto errOut
	}
	return s, e

errOut:
	return time.Now().AddDate(0, 0, 1), time.Time{}
}

func SetDQCheckedDate(s time.Time, e time.Time) error {
	cmd := fmt.Sprintf("UPDATE %s SET startdate = '%s', enddate = '%s'", CHECKED_DATE_TABLE, s.Format("20060102"), e.Format("20060102"))
	_, err := scanDB.Exec(cmd)
	return err
}

func checkDailyQuoteExist(code string, y int, m int, d int) (bool, error) {
	var id int
	cmd := fmt.Sprintf("SELECT id FROM "+STKPREFIX+code+
		" WHERE year = %d AND month = %d AND day = %d", y, m, d)
	row := scanDB.QueryRow(cmd)
	err := row.Scan(&id)

	if err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		return false, err
	} else {
		return true, nil
	}
}

func GetDailyQuote(code string, days int) (dq []DaliyQuote, err error) {
	cmd := "SELECT * FROM " + code +
		" ORDER BY year DESC, month DESC, day DESC" +
		" LIMIT " + strconv.Itoa(days)
	rows, err := scanDB.Query(cmd)
	if err != nil {
		fmt.Println("Failed to query DQ", cmd)
		return dq, nil
	}
	defer rows.Close()

	for rows.Next() {
		var r DaliyQuote
		var Id int
		err := rows.Scan(&Id, &r.Year, &r.Month, &r.Day, &r.Volume, &r.Trans, &r.Value, &r.Open, &r.High, &r.Low, &r.Close, &r.PE)
		if err != nil {
			return dq, nil
		}
		dq = append(dq, r)
	}
	return dq, err
}

func FindPrevDailyQuote(code string, y int, m int, d int) (dq DaliyQuote, err error) {
	cmd := fmt.Sprintf("SELECT * FROM "+code+
		" WHERE year < %d OR"+
		"	year = %d AND month = %d OR"+
		"	year = %d AND month = %d AND day = %d"+
		" ORDER BY year DESC, month DESC, day DESC"+
		" LIMIT 1", y, y, m, y, m, d)
	row := scanDB.QueryRow(cmd)
	var Id int
	err = row.Scan(&Id, &dq.Year, &dq.Month, &dq.Day, &dq.Volume, &dq.Trans, &dq.Value, &dq.Open, &dq.High, &dq.Low, &dq.Close, &dq.PE)
	if err != nil {
		fmt.Println("Failed to query DQ", cmd)
		return dq, nil
	}
	return dq, err
}

func AddDailyQuote(code string, dq DaliyQuote) error {
	err := checkStockTbl(code)
	if err != nil {
		return err
	}

	exist, err := checkDailyQuoteExist(code, dq.Year, dq.Month, dq.Day)
	if err != nil {
		return err
	}
	if exist {
		return nil
	}

	tblName := STKPREFIX + code
	cmd := "INSERT INTO " + tblName +
		" (year, month, day, volume, trans, value, open, high, low, close, pe)" +
		" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"

	_, err = scanDB.Exec(cmd, dq.Year, dq.Month, dq.Day, dq.Volume, dq.Trans, dq.Value, dq.Open, dq.High, dq.Low, dq.Close, dq.PE)
	return err
}
