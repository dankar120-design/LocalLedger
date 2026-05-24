package ledger

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"localledger/internal/models"
)

var (
	ErrPeriodAlreadyLocked = errors.New("period is already locked")
	ErrInvalidPeriodFormat = errors.New("invalid period format, expected YYYY-MM")
	ErrFiscalYearNotFound  = errors.New("fiscal year not found")
)

// LockPeriod låser en specifik månad (YYYY-MM) och förseglar kryptografiskt
// alla obokförda verifikationer i systemet.
func (l *Ledger) LockPeriod(yearMonth string, user string) (*models.SealResult, error) {
	// Validera format YYYY-MM
	if _, err := time.Parse("2006-01", yearMonth); err != nil {
		return nil, ErrInvalidPeriodFormat
	}

	tx, err := l.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	success := false
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()

	// Lås perioden
	_, err = tx.Exec("INSERT INTO period_locks (year_month, locked_by, locked_at) VALUES (?, ?, datetime('now', 'localtime'))", yearMonth, user)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrPeriodAlreadyLocked
		}
		return nil, fmt.Errorf("failed to insert period lock: %w", err)
	}

	if err := l.logAuditTx(tx, user, "LockPeriod", fmt.Sprintf("Locked period %s", yearMonth)); err != nil {
		return nil, err
	}

	// WORM-försegla alla obokförda verifikationer i samma transaktion
	res, err := l.sealVerificationsTx(tx, user, false)
	if err != nil {
		return nil, fmt.Errorf("failed to seal verifications: %w", err)
	}

	// Audit Log
	auditText := fmt.Sprintf("Locked period %s", yearMonth)
	_, err = tx.Exec("INSERT INTO audit_log (user, action, details) VALUES (?, 'Lock Period', ?)", user, auditText)
	if err != nil {
		return nil, fmt.Errorf("failed to write audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	success = true

	return res, nil
}

// LockFiscalYear låser ett helt räkenskapsår och förseglar kryptografiskt
// alla obokförda verifikationer i systemet.
func (l *Ledger) LockFiscalYear(id int64, user string) (*models.SealResult, error) {
	tx, err := l.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	success := false
	defer func() {
		if !success {
			tx.Rollback()
		}
	}()

	// 2. Uppdatera räkenskapsåret
	resExec, err := tx.Exec("UPDATE fiscal_years SET locked_at = datetime('now', 'localtime') WHERE id = ? AND locked_at IS NULL", id)
	if err != nil {
		return nil, fmt.Errorf("failed to update fiscal year lock: %w", err)
	}

	rowsAffected, err := resExec.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		// Antingen finns inte året, eller så är det redan låst.
		// Kör en query för att ta reda på vilket.
		var existingID int64
		err := tx.QueryRow("SELECT id FROM fiscal_years WHERE id = ?", id).Scan(&existingID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFiscalYearNotFound
		}
		return nil, ErrFiscalYearLocked
	}

	// WORM-försegla
	res, err := l.sealVerificationsTx(tx, user, false)
	if err != nil {
		return nil, fmt.Errorf("failed to seal verifications: %w", err)
	}

	// Audit Log
	auditText := fmt.Sprintf("Locked fiscal year ID %d", id)
	if err := l.logAuditTx(tx, user, "Lock Fiscal Year", auditText); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	success = true

	return res, nil
}
