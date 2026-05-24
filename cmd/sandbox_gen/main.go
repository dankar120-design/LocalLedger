package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func main() {
	companyDir := "examples/DemoForetaget_AB"
	
	// Radera hela mappen först för att garantera ett rent bygge
	os.RemoveAll(companyDir)

	err := os.MkdirAll(filepath.Join(companyDir, "underlag"), 0755)
	if err != nil {
		log.Fatal("Kunde inte skapa mappar:", err)
	}

	dbPath := filepath.Join(companyDir, "ledger.db")

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		log.Fatal("Kunde inte öppna databasen:", err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	schema := `
	DROP TABLE IF EXISTS verification_rows;
	DROP TABLE IF EXISTS verifications;
	DROP TABLE IF EXISTS audit_log;
	DROP TABLE IF EXISTS period_locks;
	DROP TABLE IF EXISTS fiscal_years;
	DROP TABLE IF EXISTS accounts;
	DROP TABLE IF EXISTS schema_version;

	CREATE TABLE schema_version (
		id INTEGER PRIMARY KEY CHECK(id = 1),
		version TEXT NOT NULL,
		app_min_version TEXT NOT NULL,
		is_sandbox BOOLEAN NOT NULL
	);

	CREATE TABLE accounts (
		code TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL
	);

	CREATE TABLE fiscal_years (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		start_date TEXT NOT NULL,
		end_date TEXT NOT NULL,
		locked_at TEXT
	);

	CREATE TABLE period_locks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		year_month TEXT NOT NULL UNIQUE,
		locked_at TEXT NOT NULL
	);

	CREATE TABLE audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TEXT NOT NULL DEFAULT (datetime('now', 'localtime')),
		user TEXT NOT NULL,
		action TEXT NOT NULL,
		details TEXT
	);

	CREATE TABLE verifications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at TEXT NOT NULL DEFAULT (datetime('now', 'localtime')),
		date TEXT NOT NULL,
		text TEXT NOT NULL,
		hash TEXT CHECK(hash IS NULL OR length(hash) = 64)
	);

	CREATE TABLE verification_rows (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		verification_id INTEGER NOT NULL,
		account TEXT NOT NULL,
		debet INTEGER NOT NULL DEFAULT 0,
		kredit INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY(verification_id) REFERENCES verifications(id),
		FOREIGN KEY(account) REFERENCES accounts(code),
		CHECK (debet >= 0 AND kredit >= 0 AND NOT (debet > 0 AND kredit > 0))
	);

	CREATE TRIGGER protect_sealed_verifications
	BEFORE UPDATE ON verifications
	FOR EACH ROW
	WHEN OLD.hash IS NOT NULL
	BEGIN
		SELECT RAISE(FAIL, 'WORM Violation: Cannot update a sealed verification.');
	END;

	CREATE TRIGGER protect_sealed_verifications_delete
	BEFORE DELETE ON verifications
	FOR EACH ROW
	WHEN OLD.hash IS NOT NULL
	BEGIN
		SELECT RAISE(FAIL, 'WORM Violation: Cannot delete a sealed verification.');
	END;

	CREATE TRIGGER protect_sealed_verification_rows_update
	BEFORE UPDATE ON verification_rows
	FOR EACH ROW
	BEGIN
		SELECT CASE
			WHEN (SELECT hash FROM verifications WHERE id = OLD.verification_id) IS NOT NULL THEN
				RAISE(FAIL, 'WORM Violation: Cannot update rows of a sealed verification.')
		END;
	END;

	CREATE TRIGGER protect_sealed_verification_rows_insert
	BEFORE INSERT ON verification_rows
	FOR EACH ROW
	BEGIN
		SELECT CASE
			WHEN (SELECT hash FROM verifications WHERE id = NEW.verification_id) IS NOT NULL THEN
				RAISE(FAIL, 'WORM Violation: Cannot insert rows into a sealed verification.')
		END;
	END;

	CREATE TRIGGER protect_sealed_verification_rows_delete
	BEFORE DELETE ON verification_rows
	FOR EACH ROW
	BEGIN
		SELECT CASE
			WHEN (SELECT hash FROM verifications WHERE id = OLD.verification_id) IS NOT NULL THEN
				RAISE(FAIL, 'WORM Violation: Cannot delete rows from a sealed verification.')
		END;
	END;
	`
	_, err = tx.Exec(schema)
	if err != nil {
		tx.Rollback()
		log.Fatal("Kunde inte skapa schema:", err)
	}

	// 1. Schema Version (Nu med tvingande ID 1 och korrekt Semver-format)
	_, err = tx.Exec("INSERT INTO schema_version (id, version, app_min_version, is_sandbox) VALUES (1, 'v1.0.0', 'v1.0.0', true)")
	if err != nil {
		tx.Rollback()
		log.Fatal(err)
	}

	// 2. Bas-Kontoplan (För FK-validering)
	accounts := map[string]struct{name, t string}{
		"1510": {"Kundfordringar", "T"},
		"1930": {"Företagskonto", "T"},
		"2440": {"Leverantörsskulder", "S"},
		"2611": {"Utgående moms, 25%", "S"},
		"3010": {"Försäljning varor, 25%", "I"},
		"4010": {"Inköp varor", "K"},
		"5010": {"Lokalhyra", "K"},
		"5410": {"Förbrukningsinventarier", "K"},
		"6110": {"Kontorsmateriel", "K"},
		"6570": {"Bankkostnader", "K"},
	}

	for code, data := range accounts {
		_, err = tx.Exec("INSERT INTO accounts (code, name, type) VALUES (?, ?, ?)", code, data.name, data.t)
		if err != nil {
			tx.Rollback()
			log.Fatal("Kunde inte inserta konto:", err)
		}
	}

	// 3. Skapa Räkenskapsår
	_, err = tx.Exec("INSERT INTO fiscal_years (start_date, end_date, locked_at) VALUES ('2023-01-01', '2023-12-31', NULL)")
	if err != nil {
		tx.Rollback()
		log.Fatal(err)
	}

	// 4. Audit Log (Vem skapade Sandboxen?)
	_, err = tx.Exec("INSERT INTO audit_log (user, action, details) VALUES ('System', 'Sandbox Initialization', 'Genererade DemoFöretaget')")
	if err != nil {
		tx.Rollback()
		log.Fatal(err)
	}

	// 5. Verifikationer (Belopp i ören, CHECK-constraint validerar)
	verifications := []struct {
		date string
		text string
		rows []map[string]interface{}
	}{
		{"2023-01-01", "Inköp av kontorsmateriel", []map[string]interface{}{{"acc": "6110", "d": 100000, "k": 0}, {"acc": "1930", "d": 0, "k": 100000}}},
		{"2023-01-02", "Försäljning tjänst", []map[string]interface{}{{"acc": "1930", "d": 500000, "k": 0}, {"acc": "3010", "d": 0, "k": 500000}}},
		{"2023-01-03", "Köp av varor", []map[string]interface{}{{"acc": "4010", "d": 200000, "k": 0}, {"acc": "1930", "d": 0, "k": 200000}}},
		{"2023-01-04", "Försäljning", []map[string]interface{}{{"acc": "1510", "d": 300000, "k": 0}, {"acc": "3010", "d": 0, "k": 300000}}},
		{"2023-01-05", "Hyra för kontor", []map[string]interface{}{{"acc": "5010", "d": 120000, "k": 0}, {"acc": "1930", "d": 0, "k": 120000}}},
		{"2023-01-06", "Köp av IT-utrustning", []map[string]interface{}{{"acc": "5410", "d": 150000, "k": 0}, {"acc": "1930", "d": 0, "k": 150000}}},
		{"2023-01-07", "Kundbetalning", []map[string]interface{}{{"acc": "1930", "d": 250000, "k": 0}, {"acc": "1510", "d": 0, "k": 250000}}},
		{"2023-01-08", "Leverantörsfaktura", []map[string]interface{}{{"acc": "2440", "d": 80000, "k": 0}, {"acc": "1930", "d": 0, "k": 80000}}},
		{"2023-01-09", "Bankavgift", []map[string]interface{}{{"acc": "6570", "d": 10000, "k": 0}, {"acc": "1930", "d": 0, "k": 10000}}},
		{"2023-01-10", "Försäljning av produkter", []map[string]interface{}{{"acc": "1930", "d": 400000, "k": 0}, {"acc": "3010", "d": 0, "k": 400000}}},
	}

	for _, v := range verifications {
		res, err := tx.Exec("INSERT INTO verifications (date, text, hash) VALUES (?, ?, NULL)", v.date, v.text)
		if err != nil {
			tx.Rollback()
			log.Fatal("Fel vid insert verification: ", err)
		}
		
		vid, err := res.LastInsertId()
		if err != nil {
			tx.Rollback()
			log.Fatal(err)
		}

		for _, row := range v.rows {
			_, err = tx.Exec("INSERT INTO verification_rows (verification_id, account, debet, kredit) VALUES (?, ?, ?, ?)",
				vid, row["acc"], row["d"], row["k"])
			if err != nil {
				tx.Rollback()
				log.Fatal("Fel vid insert rad: ", err)
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal("Kunde inte committa transaktionen:", err)
	}

	fmt.Println("Databasen och hela schemat är nu återskapat och populärt med BFL-kompatibla krav!")
}
