package ledger

import (
	"database/sql"
	"fmt"
	"strings"

	"localledger/internal/models"
)

// GetDashboardMetrics hämtar nyckeltal för Dashboarden baserat på det valda räkenskapsåret.
func (l *Ledger) GetDashboardMetrics(yearID *int64) (*models.DashboardMetrics, error) {
	var fy models.FiscalYear
	var err error
	
	if yearID != nil {
		err = l.db.QueryRow("SELECT id, start_date, end_date FROM fiscal_years WHERE id = ?", *yearID).Scan(&fy.ID, &fy.StartDate, &fy.EndDate)
	} else {
		err = l.db.QueryRow("SELECT id, start_date, end_date FROM fiscal_years ORDER BY id DESC LIMIT 1").Scan(&fy.ID, &fy.StartDate, &fy.EndDate)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			// Inga räkenskapsår = returnera nollor istället för fel
			return &models.DashboardMetrics{BankBalance: 0, NetIncome: 0}, nil
		}
		return nil, fmt.Errorf("failed to get fiscal year: %w", err)
	}

	metrics := &models.DashboardMetrics{}

	// Summerar debet och kredit grupperat per kontotyp och kontokod
	query := `
		SELECT 
			a.type,
			a.code,
			COALESCE(SUM(r.debet), 0) as total_debet,
			COALESCE(SUM(r.kredit), 0) as total_kredit
		FROM verification_rows r
		JOIN verifications v ON r.verification_id = v.id
		JOIN accounts a ON r.account = a.code
		WHERE v.date >= ? AND v.date <= ?
		GROUP BY a.type, a.code
	`

	rows, err := l.db.Query(query, fy.StartDate, fy.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query dashboard data: %w", err)
	}
	defer rows.Close()

	var totalIncome, totalExpenses int64

	for rows.Next() {
		var accType, code string
		var debet, kredit int64
		if err := rows.Scan(&accType, &code, &debet, &kredit); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Årets resultat är Intäkter minus Kostnader
		if accType == "Intäkt" {
			totalIncome += (kredit - debet)
		} else if accType == "Kostnad" {
			totalExpenses += (debet - kredit)
		}

		// Kassa/Bank är tillgångar på 19XX-konton (typiskt Debet - Kredit)
		if strings.HasPrefix(code, "19") {
			metrics.BankBalance += (debet - kredit)
		}
	}

	metrics.NetIncome = totalIncome - totalExpenses
	metrics.Income = totalIncome
	metrics.Expenses = totalExpenses

	return metrics, nil
}
