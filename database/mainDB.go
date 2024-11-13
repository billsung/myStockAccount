package myDatabase

import (
	"database/sql"
	"fmt"
	"log"
	"math"
)

const TABLENAME = "tansaction"
const HOLDING_TABLENAME = "holdings"
const REALIZED_TABLENAME = "realized"

type Transaction struct {
	Code      string  `json:"code"`
	Year      int     `json:"year"`
	Month     int     `json:"month"`
	Day       int     `json:"day"`
	Direction bool    `json:"direction"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
	Fee       int     `json:"fee"`
	Tax       int     `json:"tax"`
	Total     int     `json:"total"`
	Net       int     `json:"net"`
}

type Holding struct {
	Code     string `json:"code"`
	Year     int    `json:"year"`
	Month    int    `json:"month"`
	Day      int    `json:"day"`
	Quantity int    `json:"quantity"`
	Net      int    `json:"net"`
}

var db *sql.DB
var scanDB *sql.DB

func InitMyDB() {
	var err error
	db, err = sql.Open("sqlite3", "./database/transactionDB.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	scanDB, err = sql.Open("sqlite3", "./database/dailyDB.sqlite")
	if err != nil {
		log.Fatal(err)
	}

	initTransTbl()
	initRefTbl()
	initHoldingTbl()
	initRealizedTbl()
	initStockTbl()

	fmt.Println("Database and table initialized.")
}

func CloseMyDB() {
	db.Close()
	scanDB.Close()
}

func VacuumDB() error {
	cmd := "VACUUM"
	_, err := db.Exec(cmd)
	return err
}

func ResetTbl(tblName string) error {
	cmd := "DELETE FROM " + tblName
	_, err := db.Exec(cmd)
	if err != nil {
		return err
	}
	cmd = "UPDATE sqlite_sequence SET seq = 0 WHERE name = '" + tblName + "'"
	_, err = db.Exec(cmd)
	return err
}

func initTransTbl() {
	createTableSQL := `CREATE TABLE IF NOT EXISTS ` + TABLENAME + ` (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT NOT NULL,
		year INTEGER NOT NULL,
		month INTEGER NOT NULL,
		day INTEGER NOT NULL,
		direction BOOLEAN NOT NULL,
		price REAL NOT NULL,
		quantity INTEGER NOT NULL,
		fee INTEGER NOT NULL,
		tax INTEGER NOT NULL,
		total INTEGER NOT NULL,
		net INTEGER NOT NULL
	    );`

	if _, err := db.Exec(createTableSQL); err != nil {
		log.Fatalf("Main: Failed to create transaction table: %v", err)
	}
}

func initHoldingTbl() {
	cmd := `CREATE TABLE IF NOT EXISTS ` + HOLDING_TABLENAME + ` (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT NOT NULL,
		year INTEGER NOT NULL,
		month INTEGER NOT NULL,
		day INTEGER NOT NULL,
		quantity INTEGER NOT NULL,
		net INTEGER NOT NULL
	    );`

	if _, err := db.Exec(cmd); err != nil {
		log.Fatalf("Main: Failed to create hoildings table: %v", err)
	}
}

func initRealizedTbl() {
	cmd := `CREATE TABLE IF NOT EXISTS ` + REALIZED_TABLENAME + ` (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT NOT NULL,
		year INTEGER NOT NULL,
		month INTEGER NOT NULL,
		day INTEGER NOT NULL,
		quantity INTEGER NOT NULL,
		net INTEGER NOT NULL
	    );`

	if _, err := db.Exec(cmd); err != nil {
		log.Fatalf("Main: Failed to create realized table: %v", err)
	}
}

func AddTransaction(t Transaction) (err error) {
	t.Total = int(math.Round(t.Price * float64(t.Quantity)))

	if t.Direction {
		t.Tax = int(math.Round(float64(t.Total) * 0.003))
		t.Net = t.Total + t.Fee
	} else {
		t.Tax = 0
		t.Net = t.Total - t.Fee - t.Tax
	}

	cmd := "INSERT INTO " + TABLENAME +
		" (code, year, month, day, direction, price, quantity, fee, tax, total, net)" +
		" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"

	_, err = db.Exec(cmd, t.Code, t.Year, t.Month, t.Day, t.Direction, t.Price, t.Quantity, t.Fee, t.Tax, t.Total, t.Net)

	return err
}

func genResult(rows *sql.Rows) (transactions []Transaction, err error) {
	for rows.Next() {
		var t Transaction
		var Id int
		err := rows.Scan(&Id, &t.Code, &t.Year, &t.Month, &t.Day, &t.Direction, &t.Price, &t.Quantity, &t.Fee, &t.Tax, &t.Total, &t.Net)
		if err != nil {
			return transactions, err
		}
		transactions = append(transactions, t)
	}
	return transactions, nil
}
func genHolding(rows *sql.Rows) (holdings []Holding, err error) {
	for rows.Next() {
		var h Holding
		var Id int
		err := rows.Scan(&Id, &h.Code, &h.Year, &h.Month, &h.Day, &h.Quantity, &h.Net)
		if err != nil {
			return holdings, err
		}
		holdings = append(holdings, h)
	}
	return holdings, nil
}

func ScanTransaction() (transactions []Transaction, err error) {
	cmd := "SELECT * FROM " + TABLENAME + " ORDER BY year, month, day"
	rows, err := db.Query(cmd)
	if err != nil {
		return transactions, err
	}
	defer rows.Close()
	return genResult(rows)
}

func GetTransactions(y int, m int, d int) (transactions []Transaction, err error) {
	cmd := fmt.Sprintf("SELECT * FROM %s "+
		"WHERE year > %d OR "+
		"(year = %d AND month > %d) OR "+
		"(year = %d AND month = %d AND day <= %d)",
		TABLENAME, y, y, m, y, m, d)
	rows, err := db.Query(cmd)
	if err != nil {
		return transactions, err
	}
	defer rows.Close()
	return genResult(rows)
}

func GetHolding(c string) (hs []Holding, err error) {
	cmd := fmt.Sprintf("SELECT * FROM %s "+
		" WHERE code = '%s'"+
		" ORDER BY year, month, day",
		HOLDING_TABLENAME, c)
	rows, err := db.Query(cmd)
	if err != nil {
		return hs, err
	}
	defer rows.Close()
	return genHolding(rows)
}

func AddHolding(h Holding) error {
	cmd := "INSERT INTO " + HOLDING_TABLENAME +
		" (code, year, month, day, quantity, net)" +
		" VALUES (?, ?, ?, ?, ?, ?)"

	_, err := db.Exec(cmd, h.Code, h.Year, h.Month, h.Day, h.Quantity, h.Net)
	return err
}

func DecHolding(old Holding, nr int) (remain int, err error) {
	var cmd string
	if nr >= old.Quantity {
		cmd = fmt.Sprintf("DELETE FROM "+HOLDING_TABLENAME+
			" WHERE rowid IN (SELECT rowid"+
			"	FROM "+HOLDING_TABLENAME+
			"	WHERE code = '%s'"+
			"	ORDER BY year, month, day"+
			"	LIMIT 1)",
			old.Code)
		_, err = db.Exec(cmd)
		if err != nil {
			return -1, err
		}
		return old.Quantity, err
	}

	qty := old.Quantity - nr
	net := int(math.Round(float64(old.Net) * (float64(qty) / float64(old.Quantity))))
	cmd = fmt.Sprintf("UPDATE "+HOLDING_TABLENAME+
		" SET quantity = %d, net = %d"+
		" WHERE rowid IN (SELECT rowid"+
		"	FROM "+HOLDING_TABLENAME+
		"	WHERE code = '%s'"+
		"	ORDER BY year, month, day"+
		"	LIMIT 1)",
		qty, net, old.Code)
	_, err = db.Exec(cmd)
	if err != nil {
		return -1, err
	}
	return nr, err
}

func AddRealized(h Holding) error {
	cmd := "INSERT INTO " + REALIZED_TABLENAME +
		" (code, year, month, day, quantity, net)" +
		" VALUES (?, ?, ?, ?, ?, ?)"

	_, err := db.Exec(cmd, h.Code, h.Year, h.Month, h.Day, h.Quantity, h.Net)
	return err
}

func GetRelized(y int, m int, d int) ([]Holding, error) {
	cmd := fmt.Sprintf("SELECT * FROM %s "+
		"WHERE year > %d OR "+
		"(year = %d AND month > %d) OR "+
		"(year = %d AND month = %d AND day <= %d)",
		REALIZED_TABLENAME, y, y, m, y, m, d)
	rows, err := db.Query(cmd)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return genHolding(rows)
}
