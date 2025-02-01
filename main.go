package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mydb "myDatabase"

	_ "github.com/mattn/go-sqlite3"
)

type TextContent struct {
	Content string `json:"content"`
}
type TextContent2 struct {
	C1 string `json:"content1"`
	C2 string `json:"content2"`
}

func writeJSONOKResonse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
func writeJSONErrResonse(w http.ResponseWriter, msg string, errcode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(errcode)
	json.NewEncoder(w).Encode(map[string]string{
		"error": msg,
	})
}
func writeJSONParseIncomplete(w http.ResponseWriter, msg string, errcode int, data string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(errcode)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   msg,
		"content": data,
	})
}

func main() {
	mydb.InitMyDB()
	defer mydb.CloseMyDB()

	server := &http.Server{Addr: ":8080"}
	http.HandleFunc("/", statisticHandler)
	http.HandleFunc("/parseTrans", parseTransHandler)
	http.HandleFunc("/addref", addRefHandler)
	http.HandleFunc("/parser", parserHandler)
	http.HandleFunc("/scanner", scannerHandler)

	go func() {
		fmt.Println("Server started at http://localhost:8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("ListenAndServe error: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	<-quit
	fmt.Println("\nShutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		fmt.Printf("Server forced to shutdown: %v\n", err)
	} else {
		fmt.Println("Server exited properly")
	}
}

func parserHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		http.ServeFile(w, r, "./web/account.html")
	case "POST":
		createTransaction(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func parseTransHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		parseTransaction(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func addRefHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		var textContent TextContent2
		if err := json.NewDecoder(r.Body).Decode(&textContent); err != nil {
			writeJSONErrResonse(w, "Failed to parse request body", http.StatusBadRequest)
			return
		}
		code := textContent.C1
		name := textContent.C2
		mydb.AddRef(code, name)
		writeJSONOKResonse(w, textContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func statisticHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		http.ServeFile(w, r, "./web/index.html")
	case "POST":
		doStatistic(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func scannerHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		http.ServeFile(w, r, "./web/scanner.html")
	case "POST":
		doScan(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func createTransaction(w http.ResponseWriter, r *http.Request) {
	var t mydb.Transaction
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	t.Total = int(math.Round(t.Price * float64(t.Quantity)))
	if t.Direction {
		t.Tax = 0
		t.Net = t.Total + t.Fee
	} else {
		t.Tax = int(math.Round(float64(t.Total) * 0.003))
		t.Net = t.Total - t.Fee - t.Tax
	}

	err := mydb.AddTransaction(t)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(t)
}

func parseTransaction(w http.ResponseWriter, r *http.Request) {
	var textContent TextContent
	if err := json.NewDecoder(r.Body).Decode(&textContent); err != nil {
		writeJSONErrResonse(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	var curr_idx int
	data := textContent.Content
	data = strings.TrimPrefix(data, "\n")
	entries := strings.Split(data, "\n")
	for i, ent := range entries {
		rc, msg := parseEntry(ent)
		curr_idx = i
		if rc != http.StatusOK {
			found := true
			for n := 0; n < curr_idx && found; n = n + 1 {
				idx := strings.Index(data, "\n")
				if idx > 0 {
					data = data[idx+1:]
				}
				// fmt.Println("cut", n, idx, data)
			}
			writeJSONParseIncomplete(w, msg, rc, data)
			return
		}
	}

	writeJSONOKResonse(w, map[string]int{"count": curr_idx})
}
