package main

import (
	"database/sql"
	"fmt"
	"log"
	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "sandbox_e2e/ledger.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Println("--- PERIOD LOCKS ---")
	rows, err := db.Query("SELECT id, year_month, locked_by FROM period_locks")
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var id int
		var ym, lb string
		rows.Scan(&id, &ym, &lb)
		fmt.Printf("ID: %d | YM: %s | BY: %s\n", id, ym, lb)
	}
	rows.Close()

	fmt.Println("\n--- AUDIT LOG ---")
	rows, err = db.Query("SELECT id, action, user, details FROM audit_log")
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var id int
		var act, usr, det string
		rows.Scan(&id, &act, &usr, &det)
		fmt.Printf("ID: %d | Action: %s | User: %s | Details: %s\n", id, act, usr, det)
	}
}
