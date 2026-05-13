package ledger

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
	
	"localledger/internal/models"
)

var ErrWormViolation = errors.New("WORM Violation detected")

// GenesisHash används som utgångspunkt för den allra första verifikationen.
const GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

// SealVerifications letar upp alla obokförda verifikationer och låser dem kryptografiskt.
// Om onlyOlderThan24h är true, låses endast utkast som är äldre än 24 timmar, men
// det avbryts om det skulle lämna luckor (t.ex. om ID 10 är >24h men ID 9 är <24h).
// Eftersom WORM-kedjan måste vara sekventiell, kan vi bara låsa sekventiellt upp till
// den första posten som är nyare än 24h.
func (l *Ledger) SealVerifications(user string, onlyOlderThan24h bool) (*models.SealResult, error) {
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

	res, err := l.sealVerificationsTx(tx, user, onlyOlderThan24h)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	success = true

	return res, nil
}

// sealVerificationsTx utför WORM-låsning inuti en befintlig transaktion.
func (l *Ledger) sealVerificationsTx(tx *sql.Tx, user string, onlyOlderThan24h bool) (*models.SealResult, error) {
	// 0. Fyll eventuella luckor i ID-serien innan vi skapar WORM-kedjan
	if err := l.fillSequenceGapsTx(tx, user); err != nil {
		return nil, fmt.Errorf("failed to fill sequence gaps: %w", err)
	}

	// 1. Hämta föregående hash
	var prevHash string
	err := tx.QueryRow("SELECT hash FROM verifications WHERE hash IS NOT NULL ORDER BY id DESC LIMIT 1").Scan(&prevHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			prevHash = GenesisHash
		} else {
			return nil, fmt.Errorf("failed to fetch previous hash: %w", err)
		}
	}

	// 2. Hämta obokförda
	query := "SELECT id, date, text, attachment_hash, created_at FROM verifications WHERE hash IS NULL ORDER BY id ASC"
	rows, err := tx.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query unsealed verifications: %w", err)
	}
	defer rows.Close()

	type vData struct {
		id             int64
		date           string
		text           string
		attachmentHash sql.NullString
		createdAt      string
	}

	var toSeal []vData
	for rows.Next() {
		var v vData
		if err := rows.Scan(&v.id, &v.date, &v.text, &v.attachmentHash, &v.createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		toSeal = append(toSeal, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 2.5 Filtrera baserat på 24h-kravet om det är aktiverat
	var actuallySeal []vData
	if onlyOlderThan24h {
		twentyFourHoursAgo := time.Now().Add(-24 * time.Hour)
		for _, v := range toSeal {
			createdAtTime, err := time.ParseInLocation("2006-01-02 15:04:05", v.createdAt, time.Local)
			if err != nil {
				// Fallback om created_at har konstigt format
				createdAtTime = time.Now() 
			}
			if createdAtTime.Before(twentyFourHoursAgo) {
				actuallySeal = append(actuallySeal, v)
			} else {
				// BFL kräver sekventiell WORM-kedja. Om vi stöter på en post som är NYARE än 24h,
				// MÅSTE vi sluta försegla, annars förseglar vi ID 10 men inte ID 9, vilket bryter kedjan.
				break
			}
		}
	} else {
		actuallySeal = toSeal
	}

	if len(actuallySeal) == 0 {
		return &models.SealResult{Count: len(actuallySeal), LastHash: prevHash}, nil
	}

	rowStmt, err := tx.Prepare("SELECT account, debet, kredit FROM verification_rows WHERE verification_id = ? ORDER BY id ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare row statement: %w", err)
	}
	defer rowStmt.Close()

	updateStmt, err := tx.Prepare("UPDATE verifications SET hash = ? WHERE id = ?")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare update statement: %w", err)
	}
	defer updateStmt.Close()

	var firstID, lastID int64
	for i, v := range actuallySeal {
		if i == 0 {
			firstID = v.id
		}
		lastID = v.id

		// Bygg grundsträngen
		dataStr := fmt.Sprintf("%s|%d|%s|%s", prevHash, v.id, v.date, v.text)
		if v.attachmentHash.Valid {
			dataStr += "|" + v.attachmentHash.String
		}

		// Hämta rader
		vRows, err := rowStmt.Query(v.id)
		if err != nil {
			return nil, fmt.Errorf("failed to query rows for id %d: %w", v.id, err)
		}

		for vRows.Next() {
			var acc string
			var d, k int64
			if err := vRows.Scan(&acc, &d, &k); err != nil {
				vRows.Close()
				return nil, fmt.Errorf("failed to scan row for id %d: %w", v.id, err)
			}
			// Lägg till rad-data: |account:debet:kredit
			dataStr += fmt.Sprintf("|%s:%d:%d", acc, d, k)
		}
		vRows.Close()

		// Hasha strängen
		hashBytes := sha256.Sum256([]byte(dataStr))
		currentHash := hex.EncodeToString(hashBytes[:])

		// Uppdatera
		if _, err := updateStmt.Exec(currentHash, v.id); err != nil {
			return nil, fmt.Errorf("failed to update hash for id %d: %w", v.id, err)
		}

		prevHash = currentHash
	}

	// Audit Log (Bara om Count > 0)
	auditText := fmt.Sprintf("Sealed %d verifications (ID %d to %d)", len(actuallySeal), firstID, lastID)
	if err := l.logAuditTx(tx, user, "Seal Verifications", auditText); err != nil {
		return nil, err
	}

	return &models.SealResult{
		Count:    len(actuallySeal),
		LastHash: prevHash,
		FirstID:  firstID,
		LastID:   lastID,
	}, nil
}

// VerifyChain går igenom hela databasen från början till slut och återskapar hashen
// för att bevisa att ingenting har blivit manipulerat.
func (l *Ledger) VerifyChain() (bool, error) {
	// 1. Hämta alla låsta
	rows, err := l.db.Query("SELECT id, date, text, hash, attachment_hash, attachment_mime FROM verifications WHERE hash IS NOT NULL ORDER BY id ASC")
	if err != nil {
		return false, fmt.Errorf("failed to query sealed verifications: %w", err)
	}
	defer rows.Close()

	prevHash := GenesisHash
	
	rowStmt, err := l.db.Prepare("SELECT account, debet, kredit FROM verification_rows WHERE verification_id = ? ORDER BY id ASC")
	if err != nil {
		return false, fmt.Errorf("failed to prepare row statement: %w", err)
	}
	defer rowStmt.Close()

	for rows.Next() {
		var id int64
		var date, text, dbHash string
		var attHash, attMime sql.NullString
		if err := rows.Scan(&id, &date, &text, &dbHash, &attHash, &attMime); err != nil {
			return false, fmt.Errorf("failed to scan sealed verification: %w", err)
		}

		if attHash.Valid {
			// Snabb validering (existens på disk)
			ext := ".pdf"
			if attMime.Valid {
				if attMime.String == "image/png" {
					ext = ".png"
				} else if attMime.String == "image/jpeg" {
					ext = ".jpg"
				}
			}
			filePath := filepath.Join(l.workspacePath, "attachments", attHash.String+ext)
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				return false, fmt.Errorf("%w: attachment file missing for Verification ID %d", ErrWormViolation, id)
			}
		}

		dataStr := fmt.Sprintf("%s|%d|%s|%s", prevHash, id, date, text)
		if attHash.Valid {
			dataStr += "|" + attHash.String
		}

		// Hämta rader
		vRows, err := rowStmt.Query(id)
		if err != nil {
			return false, fmt.Errorf("failed to query rows for id %d: %w", id, err)
		}
		for vRows.Next() {
			var acc string
			var d, k int64
			if err := vRows.Scan(&acc, &d, &k); err != nil {
				vRows.Close()
				return false, fmt.Errorf("failed to scan row: %w", err)
			}
			dataStr += fmt.Sprintf("|%s:%d:%d", acc, d, k)
		}
		vRows.Close()
		
		if err := vRows.Err(); err != nil {
			return false, fmt.Errorf("error iterating rows for id %d: %w", id, err)
		}

		hashBytes := sha256.Sum256([]byte(dataStr))
		calcHash := hex.EncodeToString(hashBytes[:])

		if calcHash != dbHash {
			return false, fmt.Errorf("%w on Verification ID %d. Expected hash: %s, Found: %s", ErrWormViolation, id, calcHash, dbHash)
		}

		prevHash = calcHash
	}

	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("error iterating sealed verifications: %w", err)
	}

	return true, nil
}
