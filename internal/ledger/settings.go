package ledger

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"localledger/internal/models"
)

func (l *Ledger) GetSettings() (models.CompanySettings, error) {
	var s models.CompanySettings
	err := l.db.QueryRow("SELECT name, org_number, cloud_inbox_path, address, bankgiro, swish_number, invoice_start_number, payment_terms_days, logo_path FROM company_settings WHERE id = 1").Scan(&s.Name, &s.OrgNumber, &s.CloudInboxPath, &s.Address, &s.Bankgiro, &s.SwishNumber, &s.InvoiceStartNumber, &s.PaymentTermsDays, &s.LogoPath)
	if err != nil {
		if err == sql.ErrNoRows {
			return s, nil // Returnera tom struct om raden (av någon anledning) saknas
		}
		return s, fmt.Errorf("failed to get settings: %w", err)
	}
	return s, nil
}

func (l *Ledger) UpdateSettings(s models.CompanySettings) error {
	// Skydda mot the Ouroboros Bug (cirkulär sökväg)
	if s.CloudInboxPath != "" {
		cleanCloud := filepath.Clean(s.CloudInboxPath)
		cleanLocal := filepath.Clean(filepath.Join(l.workspacePath, "inbox"))
		if strings.EqualFold(cleanCloud, cleanLocal) {
			return fmt.Errorf("cloud inbox path cannot be exactly the same as the internal local inbox")
		}
	}

	_, err := l.db.Exec("UPDATE company_settings SET name = ?, org_number = ?, cloud_inbox_path = ?, address = ?, bankgiro = ?, swish_number = ?, invoice_start_number = ?, payment_terms_days = ?, logo_path = ? WHERE id = 1", 
		s.Name, s.OrgNumber, s.CloudInboxPath, s.Address, s.Bankgiro, s.SwishNumber, s.InvoiceStartNumber, s.PaymentTermsDays, s.LogoPath)
	if err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}
	return nil
}
