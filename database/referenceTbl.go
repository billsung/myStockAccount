package myDatabase

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
)

func initRefTbl() (err error) {
	createRefSQL := `CREATE TABLE IF NOT EXISTS reference (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT NOT NULL,
		name TEXT NOT NULL
	    );`
	if _, err = db.Exec(createRefSQL); err != nil {
		log.Fatalf("Ref: Failed to create table: %v", err)
	}
	return err
}

func RefLookupCodeByName(name string) (code string, err error) {
	query := `SELECT code FROM reference WHERE name = ?`
	row := db.QueryRow(query, name)
	err = row.Scan(&code)
	return code, err
}
func RefLookupNameByCode(code string) (name string, err error) {
	query := `SELECT name FROM reference WHERE code = ?`
	row := db.QueryRow(query, code)
	err = row.Scan(&name)
	return name, err
}
func refSetupCodeName(code string, name string) (err error) {
	_, err = RefLookupCodeByName(name)
	if err != sql.ErrNoRows {
		return errors.New("Duplicate Code")
	}

	query := `INSERT INTO reference (code, name) VALUES (?, ?)`
	_, err = db.Exec(query, code, name)
	return err
}

func AddRef(code string, name string) error {

	err := refSetupCodeName(code, name)
	if err != nil {
		fmt.Print(err.Error(), code, name)
		return err
	}
	return nil
}
