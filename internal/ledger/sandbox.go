package ledger

import (
	_ "embed"
	"fmt"
	"log"
	"path/filepath"
)

//go:embed sandbox_seed.sql
var sandboxSeedSQL string

// SeedSandbox injicerar testdata i databasen för att möjliggöra testning.
func SeedSandbox(workspace string) error {
	l, err := OpenLedger(workspace, "v3.0.0")
	if err != nil {
		return fmt.Errorf("kunde inte öppna eller migrera sandbox-db: %v", err)
	}
	defer l.Close()

	_, err = l.db.Exec(sandboxSeedSQL)
	if err != nil {
		return fmt.Errorf("kunde inte injicera seed-data: %v", err)
	}

	log.Printf("Sandbox seeded successfully at %s", filepath.Join(workspace, "ledger.db"))
	return nil
}
