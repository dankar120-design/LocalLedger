package ledger

import (
	"database/sql"
	"fmt"
	"localledger/internal/models"
)

func (l *Ledger) GetSettings() (models.CompanySettings, error) {
	var s models.CompanySettings
	err := l.db.QueryRow("SELECT name, org_number FROM company_settings WHERE id = 1").Scan(&s.Name, &s.OrgNumber)
	if err != nil {
		if err == sql.ErrNoRows {
			return s, nil // Returnera tom struct om raden (av någon anledning) saknas
		}
		return s, fmt.Errorf("failed to get settings: %w", err)
	}
	return s, nil
}

func (l *Ledger) UpdateSettings(s models.CompanySettings) error {
	_, err := l.db.Exec("UPDATE company_settings SET name = ?, org_number = ? WHERE id = 1", s.Name, s.OrgNumber)
	if err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}
	return nil
}
