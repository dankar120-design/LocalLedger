package ledger

import (
	"database/sql"
	"testing"
	_ "modernc.org/sqlite"
)

func TestMigrations_Sequential(t *testing.T) {
	// 1. Skapa en helt ny och tom minnesdatabas med Foreign Keys aktiverade i DSN
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("Failed to open memory DB: %v", err)
	}
	defer db.Close()

	// Garantera att alla anrop går mot EXAKT SAMMA in-memory databas
	db.SetMaxOpenConns(1)

	// 2. Applicera alla migreringar sekventiellt (v1 till v11)
	err = runMigrations(db)
	if err != nil {
		t.Fatalf("runMigrations failed: %v", err)
	}

	// 3. Verifiera att tabellen schema_migrations existerar och har rätt antal rader
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count schema_migrations: %v", err)
	}

	// Vi förväntar oss 13 migreringar
	expectedMigrations := 13
	if count != expectedMigrations {
		t.Errorf("Expected %d migrations, got %d", expectedMigrations, count)
	}

	// 4. Verifiera att de kritiska v11 och v13-kontona faktiskt finns
	var accountExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM accounts WHERE code = '8999')").Scan(&accountExists)
	if err != nil {
		t.Fatalf("Failed to query accounts: %v", err)
	}
	if !accountExists {
		t.Errorf("Account 8999 was not inserted by migrations")
	}

	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM accounts WHERE code = '3010')").Scan(&accountExists)
	if err != nil {
		t.Fatalf("Failed to query accounts: %v", err)
	}
	if !accountExists {
		t.Errorf("Account 3010 was not inserted by migrations")
	}

	// 5. Verifiera att WORM-skyddet (triggers) överlevde alla migreringar
	var triggerCount int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name LIKE 'protect_sealed_%'").Scan(&triggerCount)
	if err != nil {
		t.Fatalf("Failed to query triggers: %v", err)
	}
	// Vi förväntar oss exakt 5 WORM-triggers
	if triggerCount != 5 {
		t.Errorf("Expected exactly 5 WORM triggers, found %d. Migrations may have dropped them!", triggerCount)
	}
}
