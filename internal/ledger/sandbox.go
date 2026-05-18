package ledger

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log"
	"path/filepath"
)

//go:embed sandbox_seed.sql
var sandboxSeedSQL string

// SeedSandbox injicerar testdata i databasen för att möjliggöra testning.
func SeedSandbox(workspace string) error {
	dbPath := filepath.Join(workspace, "ledger.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("kunde inte öppna sandbox-db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(sandboxSeedSQL)
	if err != nil {
		return fmt.Errorf("kunde inte injicera seed-data: %v", err)
	}

	log.Printf("Sandbox seeded successfully at %s", dbPath)
	return nil
}
