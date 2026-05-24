package ledger

import (
	"fmt"

	"localledger/internal/models"
)

// GenerateOpeningBalance skapar en Ingående Balans (IB) verifikation för det nya året
// baserat på Utgående Balans (UB) från det föregående året.
func (l *Ledger) GenerateOpeningBalance(user string, fromYearID, toYearID int64) error {
	var fromStart, fromEnd string
	if err := l.db.QueryRow("SELECT start_date, end_date FROM fiscal_years WHERE id = ?", fromYearID).Scan(&fromStart, &fromEnd); err != nil {
		return fmt.Errorf("failed to get from_year: %w", err)
	}

	var toStart, toEnd string
	if err := l.db.QueryRow("SELECT start_date, end_date FROM fiscal_years WHERE id = ?", toYearID).Scan(&toStart, &toEnd); err != nil {
		return fmt.Errorf("failed to get to_year: %w", err)
	}

	// Kontrollera om IB redan finns för toYear
	var exists bool
	err := l.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM verifications 
			WHERE date >= ? AND date <= ? AND type = 'IB' AND storno_ref_id IS NULL
		)`, toStart, toEnd).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check existing IB: %w", err)
	}
	if exists {
		return fmt.Errorf("räkenskapsåret %s har redan en Ingående Balans", toStart)
	}

	// Kontrollera om toYear är låst (för säkerhets skull)
	var isLocked bool
	err = l.db.QueryRow("SELECT locked_at IS NOT NULL FROM fiscal_years WHERE id = ?", toYearID).Scan(&isLocked)
	if err == nil && isLocked {
		return ErrFiscalYearLocked
	}

	// Pre-condition: Kontrollera att hela huvudboken (klass 1-8) balanserar exakt över året.
	var totalSystemBalance int64
	err = l.db.QueryRow(`
		SELECT COALESCE(SUM(r.debet - r.kredit), 0)
		FROM verification_rows r
		JOIN verifications v ON r.verification_id = v.id
		WHERE v.date >= ? AND v.date <= ?
	`, fromStart, fromEnd).Scan(&totalSystemBalance)
	if err != nil {
		return fmt.Errorf("failed to calculate system integrity: %w", err)
	}
	if totalSystemBalance != 0 {
		return fmt.Errorf("LEDGER CORRUPTION: Huvudboken är korrupt! Summan av debet och kredit över alla konton under året är inte noll (diff: %.2f kr)", float64(totalSystemBalance)/100.0)
	}

	// Hämta UB för alla balans-konton (Klass 1 och 2)
	query := `
		SELECT 
			a.code, 
			SUM(r.debet) - SUM(r.kredit) as balance_debet
		FROM verification_rows r
		JOIN verifications v ON r.verification_id = v.id
		JOIN accounts a ON r.account = a.code
		WHERE v.date >= ? AND v.date <= ? AND (
			a.code LIKE '1%' OR a.code LIKE '2%'
		)
		GROUP BY a.code
		HAVING balance_debet != 0
	`

	rows, err := l.db.Query(query, fromStart, fromEnd)
	if err != nil {
		return fmt.Errorf("failed to query UB: %w", err)
	}
	defer rows.Close()

	var reqRows []models.RowRequest
	var totalDebet int64

	for rows.Next() {
		var code string
		var balance int64
		if err := rows.Scan(&code, &balance); err != nil {
			return fmt.Errorf("failed to scan UB row: %w", err)
		}

		totalDebet += balance

		if balance > 0 {
			reqRows = append(reqRows, models.RowRequest{
				Account: code,
				Debet:   balance,
				Kredit:  0,
			})
		} else if balance < 0 {
			reqRows = append(reqRows, models.RowRequest{
				Account: code,
				Debet:   0,
				Kredit:  -balance,
			})
		}
	}

	if len(reqRows) == 0 {
		return fmt.Errorf("finns inga balanser att överföra från föregående år")
	}

	if totalDebet != 0 {
		return fmt.Errorf("balansräkningen balanserar inte (diff: %.2f kr). Har du bokat Årets Resultat?", float64(totalDebet)/100.0)
	}

	tx, err := l.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	success := false
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()

	verification := models.VerificationRequest{
		Date: toStart, // IB bokas alltid på första dagen av det nya året
		Text: fmt.Sprintf("Ingående Balans från %s", fromStart),
		Type: "IB",
		Rows: reqRows,
	}

	vid, _, err := l.postVerificationTx(tx, user, verification)
	if err != nil {
		return fmt.Errorf("failed to post IB verification: %w", err)
	}

	if err := l.logAuditTx(tx, user, "Generate IB", fmt.Sprintf("Skapade IB %d för %s", vid.ID, toStart)); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit IB: %w", err)
	}
	success = true

	// WORM-försegling
	l.SealVerifications("System Auto-Seal (IB)", false)

	return nil
}
