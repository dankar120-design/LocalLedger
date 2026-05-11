package ledger

import (
	"fmt"
	"log"
	"strings"
	"time"

	"localledger/internal/models"
)

type VatReport struct {
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	TotalSales  int64  `json:"total_sales"`
	OutgoingVat int64  `json:"outgoing_vat"` // Utgående moms (skuld, kredit)
	IncomingVat int64  `json:"incoming_vat"` // Ingående moms (fordran, debet)
	NetVat      int64  `json:"net_vat"`      // Positiv = att betala, Negativ = få tillbaka
}

// GetVatReport genererar en summering av moms och försäljning för en given period.
func (l *Ledger) GetVatReport(startDate, endDate string) (*VatReport, error) {
	// Säkerställ att datumen tillhör samma räkenskapsår
	var startYearID, endYearID int64
	err := l.db.QueryRow("SELECT id FROM fiscal_years WHERE start_date <= ? AND end_date >= ?", startDate, startDate).Scan(&startYearID)
	if err != nil {
		return nil, fmt.Errorf("start date does not fall within any fiscal year")
	}
	err = l.db.QueryRow("SELECT id FROM fiscal_years WHERE start_date <= ? AND end_date >= ?", endDate, endDate).Scan(&endYearID)
	if err != nil {
		return nil, fmt.Errorf("end date does not fall within any fiscal year")
	}
	if startYearID != endYearID {
		return nil, fmt.Errorf("momsperioden får inte sträcka sig över ett årsskifte")
	}

	report := &VatReport{
		StartDate: startDate,
		EndDate:   endDate,
	}

	// Hämta summor för moms och försäljning
	query := `
		SELECT 
			a.code, 
			SUM(r.debet) as total_debet,
			SUM(r.kredit) as total_kredit
		FROM verification_rows r
		JOIN verifications v ON r.verification_id = v.id
		JOIN accounts a ON r.account = a.code
		WHERE v.date >= ? AND v.date <= ?
		GROUP BY a.code
		HAVING total_debet > 0 OR total_kredit > 0
	`

	rows, err := l.db.Query(query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query vat data: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var code string
		var debet, kredit int64
		if err := rows.Scan(&code, &debet, &kredit); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		balance := kredit - debet // Standrad för skulder och intäkter

		if strings.HasPrefix(code, "3") {
			report.TotalSales += balance
		} else if strings.HasPrefix(code, "261") || strings.HasPrefix(code, "262") || strings.HasPrefix(code, "263") {
			report.OutgoingVat += balance // Skuld, normalt positiv
		} else if strings.HasPrefix(code, "264") {
			// Ingående moms är fordran (debet), så vi vill ha det som positivt värde för presentationen
			report.IncomingVat += (debet - kredit)
		}
	}

	report.NetVat = report.OutgoingVat - report.IncomingVat

	return report, nil
}

// TransferVat bokför en omföring av moms till 2650 och låser perioden.
// OBS: Alla belopp i detta system hanteras som öre internt (int64). Endast presentation sker i kronor.
func (l *Ledger) TransferVat(user, startDate, endDate string) error {
	// 1. Verifiera räkenskapsår (återanvänd GetVatReport-logik indirekt)
	var fy models.FiscalYear
	err := l.db.QueryRow("SELECT id, start_date, end_date FROM fiscal_years WHERE start_date <= ? AND end_date >= ?", startDate, startDate).Scan(&fy.ID, &fy.StartDate, &fy.EndDate)
	if err != nil {
		return fmt.Errorf("start date does not fall within any fiscal year")
	}
	var endYearID int64
	err = l.db.QueryRow("SELECT id FROM fiscal_years WHERE start_date <= ? AND end_date >= ?", endDate, endDate).Scan(&endYearID)
	if err != nil {
		return fmt.Errorf("end date does not fall within any fiscal year")
	}
	if fy.ID != endYearID {
		return fmt.Errorf("momsperioden får inte sträcka sig över ett årsskifte")
	}

	// 2. Kontrollera om räkenskapsåret redan är låst
	var isLocked bool
	err = l.db.QueryRow("SELECT locked_at IS NOT NULL FROM fiscal_years WHERE id = ?", fy.ID).Scan(&isLocked)
	if err == nil && isLocked {
		return ErrFiscalYearLocked
	}

	// 3. Hämta alla momskonton som har saldo
	query := `
		SELECT 
			a.code, 
			SUM(r.debet) - SUM(r.kredit) as balance_debet -- Positivt = Debetsaldo, Negativt = Kreditsaldo
		FROM verification_rows r
		JOIN verifications v ON r.verification_id = v.id
		JOIN accounts a ON r.account = a.code
		WHERE v.date >= ? AND v.date <= ? AND (
			a.code LIKE '261%' OR 
			a.code LIKE '262%' OR 
			a.code LIKE '263%' OR 
			a.code LIKE '264%'
		)
		GROUP BY a.code
		HAVING balance_debet != 0
	`

	rows, err := l.db.Query(query, startDate, endDate)
	if err != nil {
		return fmt.Errorf("failed to query vat balances: %w", err)
	}
	defer rows.Close()

	var verificationRows []models.RowRequest
	var netVatBalance int64 // Samlad effekt mot 2650

	for rows.Next() {
		var code string
		var balanceDebet int64
		if err := rows.Scan(&code, &balanceDebet); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		if balanceDebet > 0 {
			// Kontot har debetsaldo (t.ex. ingående moms). Krediteras för att nollställa.
			verificationRows = append(verificationRows, models.RowRequest{
				Account: code,
				Kredit:  balanceDebet,
			})
			netVatBalance -= balanceDebet
		} else if balanceDebet < 0 {
			// Kontot har kreditsaldo (t.ex. utgående moms). Debiteras för att nollställa.
			verificationRows = append(verificationRows, models.RowRequest{
				Account: code,
				Debet:   -balanceDebet,
			})
			netVatBalance += (-balanceDebet)
		}
	}

	if len(verificationRows) == 0 {
		return fmt.Errorf("inga momssaldon att omföra för denna period")
	}

	// Boka nettot mot 2650
	if netVatBalance > 0 {
		// Positivt netVatBalance betyder mer debet (Utgående moms nollställd > Ingående moms nollställd). 2650 krediteras (Skuld till SKV).
		verificationRows = append(verificationRows, models.RowRequest{
			Account: "2650",
			Kredit:  netVatBalance,
		})
	} else if netVatBalance < 0 {
		// 2650 debiteras (Fordran på SKV)
		verificationRows = append(verificationRows, models.RowRequest{
			Account: "2650",
			Debet:   -netVatBalance,
		})
	}

	// Starta den atomära transaktionen!
	tx, err := l.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	success := false
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()

	// 4. Skapa verifikationen
	verification := models.VerificationRequest{
		Date: endDate,
		Text: fmt.Sprintf("Momsomföring %s - %s", startDate, endDate),
		Rows: verificationRows,
	}

	_, _, err = l.postVerificationTx(tx, user, verification)
	if err != nil {
		return fmt.Errorf("failed to post vat transfer verification: %w", err)
	}

	// 5. Lås alla månader i perioden (inkluderar själva verifikationen vi nyss bokförde)
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return fmt.Errorf("invalid start date format: %w", err)
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return fmt.Errorf("invalid end date format: %w", err)
	}

	// Säkerställ att start alltid ligger först (för day 1 normalization)
	start = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())

	for d := start; d.Before(end) || d.Equal(end); d = d.AddDate(0, 1, 0) {
		yearMonth := d.Format("2006-01")
		
		// Insert or ignore period lock
		_, err := tx.Exec(`
			INSERT INTO period_locks (year_month, locked_by, locked_at) 
			VALUES (?, ?, datetime('now', 'localtime'))
			ON CONFLICT(year_month) DO NOTHING
		`, yearMonth, user)
		if err != nil {
			return fmt.Errorf("failed to lock period %s: %w", yearMonth, err)
		}
	}

	if err := l.logAuditTx(tx, user, "TransferVat", fmt.Sprintf("Momsomföring %s till %s", startDate, endDate)); err != nil {
		return err
	}

	// Commit the atomic change!
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit vat transfer: %w", err)
	}
	success = true

	// 6. WORM-försegla hela databasen (alla obokförda fram till nu) för att säkerställa compliance
	// Detta sker utanför SQLite-transaktionen för att undvika Deadlock ifall Seal skapar en egen tx.
	// Om detta misslyckas är ingen skada skedd, verifikationen är postad och perioden låst.
	_, err = l.SealVerifications("System Auto-Seal (Moms)", false)
	if err != nil {
		// Logga varningen men faila inte requestet eftersom BFL-omföringen är säkrad lokalt
		log.Printf("Warning: Failed to auto-seal after VAT transfer: %v", err)
	}

	return nil
}
