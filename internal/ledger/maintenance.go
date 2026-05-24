package ledger

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// FillSequenceGaps letar upp luckor i verifikationsnummerserien och fyller dem med makuleringsposter.
func (l *Ledger) FillSequenceGaps(user string) error {
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

	if err := l.fillSequenceGapsTx(tx, user); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	success = true
	return nil
}

func (l *Ledger) fillSequenceGapsTx(tx *sql.Tx, user string) error {
	// 1. Hämta max ID
	var maxID sql.NullInt64
	err := tx.QueryRow("SELECT MAX(id) FROM verifications").Scan(&maxID)
	if err != nil {
		return fmt.Errorf("failed to fetch max ID: %w", err)
	}
	if !maxID.Valid {
		return nil // Inga verifikationer finns, inga luckor att fylla
	}

	// 2. Hämta alla befintliga IDn
	rows, err := tx.Query("SELECT id FROM verifications")
	if err != nil {
		return fmt.Errorf("failed to fetch existing IDs: %w", err)
	}
	defer rows.Close()

	existingIDs := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan ID: %w", err)
		}
		existingIDs[id] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Vi skapar makuleringsposterna med dagens datum
	today := time.Now().Format("2006-01-02")

	// Kolla om det finns luckor innan vi validerar lås
	hasGaps := false
	gapCount := 0
	for i := int64(1); i < maxID.Int64; i++ {
		if !existingIDs[i] {
			hasGaps = true
			gapCount++
			if gapCount > 1000 {
				return fmt.Errorf("sequence poisoning detected: >1000 gaps found. operation aborted to prevent database bloat")
			}
		}
	}

	if !hasGaps {
		return nil
	}

	// 3. Lås-kontroll för dagens datum
	// Kontrollera Fiscal Year
	var fyLockedAt sql.NullString
	err = tx.QueryRow("SELECT locked_at FROM fiscal_years WHERE start_date <= ? AND end_date >= ?", today, today).Scan(&fyLockedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoFiscalYear
		}
		return fmt.Errorf("failed to query fiscal year: %w", err)
	}
	if fyLockedAt.Valid {
		return ErrFiscalYearLocked
	}

	// Kontrollera Period Lock
	periodStr := today[:7]
	var plLockedAt string
	err = tx.QueryRow("SELECT locked_at FROM period_locks WHERE year_month = ?", periodStr).Scan(&plLockedAt)
	if err == nil {
		return ErrPeriodLocked
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to query period locks: %w", err)
	}

	// 4. Hitta och fyll luckor
	gapsFilled := 0
	for i := int64(1); i < maxID.Int64; i++ {
		if !existingIDs[i] {
			text := "Makulerad p.g.a. tekniskt avbrott i sekvens"
			_, err = tx.Exec("INSERT INTO verifications (id, date, text, hash) VALUES (?, ?, ?, NULL)", i, today, text)
			if err != nil {
				return fmt.Errorf("failed to insert gap verification %d: %w", i, err)
			}
			gapsFilled++
		}
	}

	if gapsFilled > 0 {
		auditText := fmt.Sprintf("Makulerade %d saknade verifikationsnummer för att laga serien", gapsFilled)
		_, err = tx.Exec("INSERT INTO audit_log (user, action, details) VALUES (?, 'Fill Gaps', ?)", user, auditText)
		if err != nil {
			return fmt.Errorf("failed to insert audit log: %w", err)
		}
	}

	return nil
}
