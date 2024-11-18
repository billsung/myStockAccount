package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	mydb "myDatabase"
)

func parseEntry(entry string) (errcode int, msg string) {
	parts := strings.Split(entry, "\t")
	if len(parts) < 7 {
		return http.StatusBadRequest, "Invalid input."
	}

	datePart := parts[0]
	directionPart := parts[1]
	namePart := parts[2]
	pricePart := parts[3]
	quantityPart := parts[4]
	feePart := parts[6]

	y := time.Now().Year()
	// y := time.Now().AddDate(-4, 0, 0).Year()
	date, err := time.Parse("0102", datePart)
	if err != nil {
		date, err = time.Parse("20060102", datePart)
		if err != nil {
			return http.StatusBadRequest, "Invalid date format"
		}
	}
	_, m, d := date.Date()

	// Direction: Check if direction contains "買"
	direction := strings.Contains(directionPart, "買")

	// Name to Code: Lookup code by name in the reference database
	code, err := mydb.RefLookupCodeByName(namePart)
	if err != nil {
		msg := "Failed to find code for name " + namePart
		return http.StatusBadRequest, msg
	}

	// Price: Convert to float64
	price, err := strconv.ParseFloat(strings.Replace(pricePart, ",", "", -1), 64)
	if err != nil {
		return http.StatusBadRequest, "Invalid price format"
	}

	// Quantity: Convert to int
	quantity, err := strconv.Atoi(strings.Replace(quantityPart, ",", "", -1))
	if err != nil {
		return http.StatusBadRequest, "Invalid quantity format"
	}

	// Fee: Convert to int
	fee, err := strconv.Atoi(strings.Replace(feePart, ",", "", -1))
	if err != nil {
		return http.StatusBadRequest, "Invalid fee format"
	}

	// Create Transaction instance
	transaction := mydb.Transaction{
		Year:      y,
		Month:     int(m),
		Day:       d,
		Direction: direction,
		Code:      code,
		Price:     price,
		Quantity:  quantity,
		Fee:       fee,
	}

	err = mydb.AddTransaction(transaction)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	err = procTrans(transaction)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	return http.StatusOK, ""
}
