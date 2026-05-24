package ledger

import (
	"database/sql"
	"fmt"
)

// logAuditTx sparar en händelse i audit_log tabellen med hjälp av en pågående transaktion.
func (l *Ledger) logAuditTx(tx *sql.Tx, user, action, details string) error {
	_, err := tx.Exec("INSERT INTO audit_log (user, action, details) VALUES (?, ?, ?)", user, action, details)
	if err != nil {
		return fmt.Errorf("failed to log audit event %s: %w", action, err)
	}
	return nil
}

// logAudit sparar en händelse i audit_log tabellen utanför en transaktion.
func (l *Ledger) logAudit(user, action, details string) error {
	_, err := l.db.Exec("INSERT INTO audit_log (user, action, details) VALUES (?, ?, ?)", user, action, details)
	if err != nil {
		return fmt.Errorf("failed to log audit event %s: %w", action, err)
	}
	return nil
}
