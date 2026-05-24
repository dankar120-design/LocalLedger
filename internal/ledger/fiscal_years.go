package ledger

import (
	"fmt"
	"time"

	"localledger/internal/models"
)

// GetFiscalYears returnerar alla räkenskapsår
func (l *Ledger) GetFiscalYears() ([]models.FiscalYear, error) {
	rows, err := l.db.Query("SELECT id, start_date, end_date, locked_at IS NOT NULL as is_locked FROM fiscal_years ORDER BY start_date DESC")
	if err != nil {
		return nil, fmt.Errorf("failed to query fiscal years: %w", err)
	}
	defer rows.Close()

	var years []models.FiscalYear
	for rows.Next() {
		var fy models.FiscalYear
		if err := rows.Scan(&fy.ID, &fy.StartDate, &fy.EndDate, &fy.IsLocked); err != nil {
			return nil, fmt.Errorf("failed to scan fiscal year: %w", err)
		}
		years = append(years, fy)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return years, nil
}

// CreateFiscalYear skapar ett nytt räkenskapsår och för över utgående balanser till en öppningsverifikation
func (l *Ledger) CreateFiscalYear(user string) (*models.FiscalYear, error) {
	// 1. Hämta det senaste räkenskapsåret
	var lastYear models.FiscalYear
	err := l.db.QueryRow("SELECT id, start_date, end_date, locked_at IS NOT NULL FROM fiscal_years ORDER BY id DESC LIMIT 1").
		Scan(&lastYear.ID, &lastYear.StartDate, &lastYear.EndDate, &lastYear.IsLocked)
	if err != nil {
		return nil, fmt.Errorf("kunde inte hitta senaste räkenskapsåret: %w", err)
	}

	// 2. Beräkna datum för det nya året (ex. +1 år)
	startDate, err := time.Parse("2006-01-02", lastYear.StartDate)
	if err != nil {
		return nil, fmt.Errorf("ogiltigt datumformat på start_date: %w", err)
	}
	endDate, err := time.Parse("2006-01-02", lastYear.EndDate)
	if err != nil {
		return nil, fmt.Errorf("ogiltigt datumformat på end_date: %w", err)
	}

	newStartDate := startDate.AddDate(1, 0, 0).Format("2006-01-02")
	newEndDate := endDate.AddDate(1, 0, 0).Format("2006-01-02")

	// 3. Påbörja transaktion
	tx, err := l.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	success := false
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()

	// 4. Hämta balanser för det gamla året (UB)
	// Vi hämtar bara Tillgångar och Skulder (Balanskonton: börjar på 1 eller 2)
	query := `
		SELECT 
			a.code, 
			a.type,
			SUM(r.debet) as total_debet,
			SUM(r.kredit) as total_kredit
		FROM verification_rows r
		JOIN verifications v ON r.verification_id = v.id
		JOIN accounts a ON r.account = a.code
		WHERE v.date >= ? AND v.date <= ?
		GROUP BY a.code, a.type
	`
	rows, err := tx.Query(query, lastYear.StartDate, lastYear.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query balances: %w", err)
	}
	
	var totalIncome, totalExpenses int64
	var ibRows []models.RowRequest

	for rows.Next() {
		var code, accType string
		var debet, kredit int64
		if err := rows.Scan(&code, &accType, &debet, &kredit); err != nil {
			rows.Close()
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if accType == "Tillgång" || accType == "Skuld" {
			netDebet := debet - kredit
			if netDebet > 0 {
				ibRows = append(ibRows, models.RowRequest{Account: code, Debet: netDebet, Kredit: 0})
			} else if netDebet < 0 {
				ibRows = append(ibRows, models.RowRequest{Account: code, Debet: 0, Kredit: -netDebet})
			}
		} else if accType == "Intäkt" {
			totalIncome += (kredit - debet)
		} else if accType == "Kostnad" {
			totalExpenses += (debet - kredit)
		}
	}
	rows.Close()

	// 5. Beräkna Årets Resultat och boka mot Eget Kapital (2010)
	netIncome := totalIncome - totalExpenses
	if netIncome > 0 {
		// Vinst ökar Eget Kapital (Kredit)
		ibRows = append(ibRows, models.RowRequest{Account: "2010", Debet: 0, Kredit: netIncome})
	} else if netIncome < 0 {
		// Förlust minskar Eget Kapital (Debet)
		ibRows = append(ibRows, models.RowRequest{Account: "2010", Debet: -netIncome, Kredit: 0})
	}

	// Vi letar nu upp netto per konto ifall "2010" redan fanns i listan.
	// Egentligen är det renare att aggregera allt i en map.
	aggregatedRows := make(map[string]int64) // Net debet per konto
	for _, r := range ibRows {
		aggregatedRows[r.Account] += (r.Debet - r.Kredit)
	}

	// 6. Skapa det nya året
	resExec, err := tx.Exec("INSERT INTO fiscal_years (start_date, end_date) VALUES (?, ?)", newStartDate, newEndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to insert new fiscal year: %w", err)
	}
	newFyID, err := resExec.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get new fiscal year id: %w", err)
	}

	// 7. Skapa Ingående Balans-verifikation (om det finns rader)
	if len(aggregatedRows) > 0 {
		vRes, err := tx.Exec("INSERT INTO verifications (date, text, created_at) VALUES (?, 'Ingående Balans', datetime('now', 'localtime'))", newStartDate)
		if err != nil {
			return nil, fmt.Errorf("failed to insert IB verification: %w", err)
		}
		vID, _ := vRes.LastInsertId()

		for code, netDebet := range aggregatedRows {
			if netDebet == 0 {
				continue
			}
			var debet, kredit int64
			if netDebet > 0 {
				debet = netDebet
			} else {
				kredit = -netDebet
			}
			_, err = tx.Exec("INSERT INTO verification_rows (verification_id, account, debet, kredit) VALUES (?, ?, ?, ?)", vID, code, debet, kredit)
			if err != nil {
				return nil, fmt.Errorf("failed to insert IB row: %w", err)
			}
		}
	}

	// 8. Audit Log
	auditText := fmt.Sprintf("Skapade nytt räkenskapsår %s till %s", newStartDate, newEndDate)
	if err := l.logAuditTx(tx, user, "Create Fiscal Year", auditText); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	success = true

	return &models.FiscalYear{
		ID:        newFyID,
		StartDate: newStartDate,
		EndDate:   newEndDate,
		IsLocked:  false,
	}, nil
}
