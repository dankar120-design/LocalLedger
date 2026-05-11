package ledger

import (
	"fmt"
	"regexp"
)

var validAccountRegex = regexp.MustCompile(`^[1-8][0-9]{3}$`)

// AddAccount lägger till ett nytt konto i kontoplanen.
func (l *Ledger) AddAccount(code, name, accType string) error {
	// 1. Validera koden
	if !validAccountRegex.MatchString(code) {
		return fmt.Errorf("invalid account code: must be exactly 4 digits starting with 1-8")
	}

	// 2. Validera typen
	switch accType {
	case "Tillgång", "Skuld", "Intäkt", "Kostnad":
		// OK
	default:
		return fmt.Errorf("invalid account type: %s", accType)
	}

	// 3. Kontrollera om kontot redan finns
	var exists bool
	err := l.db.QueryRow("SELECT EXISTS(SELECT 1 FROM accounts WHERE code = ?)", code).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check account existence: %w", err)
	}
	if exists {
		return fmt.Errorf("account %s already exists", code)
	}

	// 4. Lägg till kontot
	_, err = l.db.Exec("INSERT INTO accounts (code, name, type) VALUES (?, ?, ?)", code, name, accType)
	if err != nil {
		return fmt.Errorf("failed to insert account: %w", err)
	}

	return nil
}
