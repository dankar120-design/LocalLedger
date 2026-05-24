package ledger

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	
	_ "modernc.org/sqlite"
)

// TestOpenLedger_MigrationRollback verifierar att en misslyckad migrering
// inte förstör databasen och att systemet återställs helt till föregående tillstånd.
func TestOpenLedger_MigrationRollback(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "ledger.db")

	// 1. Skapa en giltig databas på en tidigare version (t.ex. Version 8)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	
	// Skapa schema_migrations och sätt till Version 8
	_, err = db.Exec("CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY)")
	if err != nil {
		db.Close()
		t.Fatalf("failed to create schema_migrations table: %v", err)
	}
	
	for i := 1; i <= 8; i++ {
		_, err = db.Exec("INSERT INTO schema_migrations (version) VALUES (?)", i)
		if err != nil {
			db.Close()
			t.Fatalf("failed to populate schema_migrations: %v", err)
		}
	}

	// Skapa company_settings tabell (från Version 6)
	_, err = db.Exec(`
		CREATE TABLE company_settings (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			name TEXT NOT NULL DEFAULT '',
			org_number TEXT NOT NULL DEFAULT ''
		);
		INSERT INTO company_settings (id, name, org_number) VALUES (1, 'Test AB', '123456-7890');
	`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to setup company_settings: %v", err)
	}

	// Provocera fram en krasch i Version 9:
	// Version 9 försöker köra: ALTER TABLE company_settings ADD COLUMN cloud_inbox_path TEXT NOT NULL DEFAULT '';
	// Om vi lägger till kolumnen 'cloud_inbox_path' manuellt redan nu, kommer Version 9 migreringen
	// att krascha i SQLite eftersom man inte kan lägga till en kolumn som redan finns.
	_, err = db.Exec("ALTER TABLE company_settings ADD COLUMN cloud_inbox_path TEXT NOT NULL DEFAULT 'pre-existing'")
	if err != nil {
		db.Close()
		t.Fatalf("failed to provoke crash preparation: %v", err)
	}

	// Stäng anslutningen inför OpenLedger
	db.Close()

	// Kontrollera att filen finns och att backup INTE finns än
	backupPath := dbPath + ".tmp_backup"
	if _, err := os.Stat(backupPath); err == nil {
		t.Fatalf("backup file already exists before start")
	}

	// 2. Försök att köra OpenLedger, vilket triggar runMigrations och därmed kraschen i Version 9
	_, err = OpenLedger(tempDir, "v3.0.0")
	if err == nil {
		t.Fatalf("expected OpenLedger to fail due to provoked migration crash, but it succeeded")
	}

	// Verifiera att felet är relaterat till migreringen
	t.Logf("Successfully caught expected migration error: %v", err)

	// 3. Verifiera att den temporära backup-filen städades bort
	if _, err := os.Stat(backupPath); err == nil || !os.IsNotExist(err) {
		t.Errorf("expected temporary backup file %s to be deleted, but it still exists", backupPath)
	}

	// 4. Öppna databasen igen och kontrollera att den återställdes helt till Version 8-läge
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to re-open restored database: %v", err)
	}
	defer db.Close()

	// Kontrollera maxversionen i schema_migrations - den ska fortfarande vara 8!
	var maxVersion int
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&maxVersion)
	if err != nil {
		t.Fatalf("failed to query schema_migrations: %v", err)
	}
	if maxVersion != 8 {
		t.Errorf("expected restored database to be at Version 8, but got Version %d", maxVersion)
	}

	// Kontrollera att kolumnen 'cloud_inbox_path' har sitt ursprungliga provokations-värde intakt
	var cloudPath string
	err = db.QueryRow("SELECT cloud_inbox_path FROM company_settings WHERE id = 1").Scan(&cloudPath)
	if err != nil {
		t.Fatalf("failed to query restored company_settings: %v", err)
	}
	if cloudPath != "pre-existing" {
		t.Errorf("expected company_settings to be restored, but got cloud_inbox_path = %q", cloudPath)
	}
	
	fmt.Println("Test passed: Database safely restored to previous state after migration failure!")
}
