package ledger

import (
	"fmt"
	"log"
	"time"

	"localledger/internal/models"
)

type VatBoxes struct {
	Box05 int64 `json:"box_05"` // Försäljning 25% (3000, 3001, 3010, 3011, 3020, 3040, 3041)
	Box06 int64 `json:"box_06"` // Försäljning 12% (3002, 3042)
	Box07 int64 `json:"box_07"` // Försäljning 6% (3003, 3043)
	Box10 int64 `json:"box_10"` // Utgående moms 25% (2610, 2611)
	Box11 int64 `json:"box_11"` // Utgående moms 12% (2620)
	Box12 int64 `json:"box_12"` // Utgående moms 6% (2630)
	Box20 int64 `json:"box_20"` // Inköp varor EU (4515)
	Box21 int64 `json:"box_21"` // Inköp tjänster EU (4531)
	Box30 int64 `json:"box_30"` // Utgående moms på EU-förvärv (2614)
	Box48 int64 `json:"box_48"` // Ingående moms (2640, 2641, 2645)
}

type VatReport struct {
	StartDate   string   `json:"start_date"`
	EndDate     string   `json:"end_date"`
	TotalSales  int64    `json:"total_sales"`  // Bevarad för UI-bakåtkompatibilitet
	OutgoingVat int64    `json:"outgoing_vat"` // Bevarad för UI-bakåtkompatibilitet
	IncomingVat int64    `json:"incoming_vat"` // Bevarad för UI-bakåtkompatibilitet
	NetVat      int64    `json:"net_vat"`      // Bevarad för UI-bakåtkompatibilitet
	Boxes       VatBoxes `json:"boxes"`
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

	// Vi hämtar exakt de konton vi har strikt stöd för i svensk momsredovisning.
	// Försäljningskonton täcker standardscenarier, inklusive provisionerade konton.
	query := `
		SELECT 
			a.code, 
			SUM(r.debet) as total_debet,
			SUM(r.kredit) as total_kredit
		FROM verification_rows r
		JOIN verifications v ON r.verification_id = v.id
		JOIN accounts a ON r.account = a.code
		WHERE v.date >= ? AND v.date <= ?
		  AND a.code IN (
			'3000', '3001', '3002', '3003', '3010', '3011', '3020', '3040', '3041', '3042', '3043',
			'4515', '4531',
			'2610', '2611', '2614', '2620', '2621', '2630', '2631', '2640', '2641', '2645'
		  )
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

		balance := kredit - debet     // Standard för skulder och intäkter
		costBalance := debet - kredit // Standard för kostnader och tillgångar

		switch code {
		// Ruta 05: Försäljning 25% (inkl. standard varuförsäljning 3010, 3020)
		case "3000", "3001", "3010", "3011", "3020", "3040", "3041":
			report.Boxes.Box05 += balance
			report.TotalSales += balance
		// Ruta 06: Försäljning 12%
		case "3002", "3042":
			report.Boxes.Box06 += balance
			report.TotalSales += balance
		// Ruta 07: Försäljning 6%
		case "3003", "3043":
			report.Boxes.Box07 += balance
			report.TotalSales += balance

		// Ruta 20, 21: EU-inköp (Redovisas som positivt inköpsbelopp)
		case "4515":
			report.Boxes.Box20 += costBalance
		case "4531":
			report.Boxes.Box21 += costBalance

		// Ruta 10: Utgående moms 25%
		case "2610", "2611":
			report.Boxes.Box10 += balance
			report.OutgoingVat += balance
		// Ruta 11: Utgående moms 12%
		case "2620", "2621":
			report.Boxes.Box11 += balance
			report.OutgoingVat += balance
		// Ruta 12: Utgående moms 6%
		case "2630", "2631":
			report.Boxes.Box12 += balance
			report.OutgoingVat += balance

		// Ruta 30: Utgående moms omvänd skattskyldighet
		case "2614":
			report.Boxes.Box30 += balance
			report.OutgoingVat += balance

		// Ruta 48: Ingående moms
		case "2640", "2641", "2645":
			report.Boxes.Box48 += costBalance
			report.IncomingVat += costBalance
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over vat report rows: %w", err)
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

	// 3. Hämta alla momskonton som har saldo baserat på strikt godkända momskonton.
	query := `
		SELECT 
			a.code, 
			SUM(r.debet) - SUM(r.kredit) as balance_debet -- Positivt = Debetsaldo, Negativt = Kreditsaldo
		FROM verification_rows r
		JOIN verifications v ON r.verification_id = v.id
		JOIN accounts a ON r.account = a.code
		WHERE v.date >= ? AND v.date <= ? AND a.code IN ('2610', '2611', '2614', '2620', '2621', '2630', '2631', '2640', '2641', '2645')
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

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating over vat balance rows: %w", err)
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
