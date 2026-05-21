package ledger

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"localledger/internal/models"
)

// StornoVerification skapar en Total Reversering (stornopost) av en given verifikation.
func (l *Ledger) StornoVerification(originalID int64, user string) (*models.VerificationResult, error) {
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

	// 1. Kontrollera om den redan är stornerad
	var isStornoed int
	err = tx.QueryRow("SELECT 1 FROM verifications WHERE storno_ref_id = ?", originalID).Scan(&isStornoed)
	if err == nil {
		return nil, fmt.Errorf("%w: verification %d is already stornoed", ErrValidation, originalID)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check storno status: %w", err)
	}

	// 2. Läs originalposten
	var origDate, origText string
	var origStornoRefID sql.NullInt64
	err = tx.QueryRow("SELECT date, text, storno_ref_id FROM verifications WHERE id = ?", originalID).Scan(&origDate, &origText, &origStornoRefID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("original verification %d does not exist", originalID)
		}
		return nil, fmt.Errorf("failed to fetch original verification: %w", err)
	}

	if origStornoRefID.Valid {
		return nil, fmt.Errorf("%w: cannot storno a verification that is itself a storno (refers to A%d)", ErrValidation, origStornoRefID.Int64)
	}

	// 2. Läs originalraderna
	rows, err := tx.Query("SELECT account, debet, kredit FROM verification_rows WHERE verification_id = ?", originalID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch original rows: %w", err)
	}
	defer rows.Close()

	var origRows []models.RowResponse
	for rows.Next() {
		var r models.RowResponse
		if err := rows.Scan(&r.Account, &r.Debet, &r.Kredit); err != nil {
			return nil, fmt.Errorf("failed to scan original row: %w", err)
		}
		origRows = append(origRows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(origRows) == 0 {
		return nil, fmt.Errorf("%w: cannot storno a verification with no rows (gap filler)", ErrValidation)
	}

	// 4. Datum för stornoposten = IDAG
	today := time.Now().Format("2006-01-02")

	// 5. Lås-kontroll för dagens datum
	// Kontrollera Fiscal Year
	var fyLockedAt sql.NullString
	err = tx.QueryRow("SELECT locked_at FROM fiscal_years WHERE start_date <= ? AND end_date >= ?", today, today).Scan(&fyLockedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoFiscalYear
		}
		return nil, fmt.Errorf("failed to query fiscal year: %w", err)
	}
	if fyLockedAt.Valid {
		return nil, ErrFiscalYearLocked
	}

	// Kontrollera Period Lock
	periodStr := today[:7]
	var plLockedAt string
	err = tx.QueryRow("SELECT locked_at FROM period_locks WHERE year_month = ?", periodStr).Scan(&plLockedAt)
	if err == nil {
		return nil, ErrPeriodLocked
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query period locks: %w", err)
	}

	// 6. Skapa den nya verifikationen
	newText := fmt.Sprintf("Reversering av A%d", originalID)
	var newID int64
	var createdAt string
	err = tx.QueryRow("INSERT INTO verifications (date, text, type, hash, storno_ref_id) VALUES (?, ?, 'STORNO', NULL, ?) RETURNING id, created_at", today, newText, originalID).Scan(&newID, &createdAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("%w: verification %d is already stornoed", ErrValidation, originalID)
		}
		return nil, fmt.Errorf("failed to insert storno verification: %w", err)
	}

	// 7. Sätt in rader med omvänd debet/kredit
	for _, row := range origRows {
		_, err = tx.Exec("INSERT INTO verification_rows (verification_id, account, debet, kredit) VALUES (?, ?, ?, ?)",
			newID, row.Account, row.Kredit, row.Debet) // Byter plats på debet och kredit
		if err != nil {
			return nil, fmt.Errorf("failed to insert storno row for account %s: %w", row.Account, err)
		}
	}

	// 8. Audit Log
	auditText := fmt.Sprintf("Skapade stornopost A%d för originalverifikation A%d", newID, originalID)
	if err := l.logAuditTx(tx, user, "Storno Verification", auditText); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	success = true

	return &models.VerificationResult{
		ID:        newID,
		CreatedAt: createdAt,
	}, nil
}
