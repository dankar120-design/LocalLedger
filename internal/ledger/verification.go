package ledger

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"localledger/internal/models"
)

var (
	ErrValidation       = errors.New("Valideringsfel")
	ErrNoFiscalYear     = errors.New("Inget aktivt räkenskapsår hittades för detta datum")
	ErrFiscalYearLocked = errors.New("Räkenskapsåret är låst")
	ErrPeriodLocked     = errors.New("Bokföringsperioden (månaden) är låst")
)

// PostVerification validerar och bokför en verifikation atomärt.
func (l *Ledger) PostVerification(user string, req models.VerificationRequest) (*models.VerificationResult, error) {
	// 1. Go-validering
	if _, err := time.Parse("2006-01-02", req.Date); err != nil {
		return nil, fmt.Errorf("%w: ogiltigt datumformat (måste vara ÅÅÅÅ-MM-DD)", ErrValidation)
	}

	if len(req.Rows) < 2 {
		return nil, fmt.Errorf("%w: en verifikation måste ha minst 2 rader", ErrValidation)
	}

	var sumDebet, sumKredit int64
	for i, row := range req.Rows {
		if row.Debet < 0 || row.Kredit < 0 {
			return nil, fmt.Errorf("%w: rad %d har negativt belopp (Debet: %d, Kredit: %d)", ErrValidation, i+1, row.Debet, row.Kredit)
		}
		if row.Debet == 0 && row.Kredit == 0 {
			return nil, fmt.Errorf("%w: rad %d har nollbelopp (måste ha antingen debet eller kredit > 0)", ErrValidation, i+1)
		}
		if row.Debet > 0 && row.Kredit > 0 {
			return nil, fmt.Errorf("%w: rad %d kan inte ha både debet och kredit > 0", ErrValidation, i+1)
		}
		sumDebet += row.Debet
		sumKredit += row.Kredit
	}

	if sumDebet != sumKredit {
		return nil, fmt.Errorf("%w: verifikationen balanserar inte (Debet: %d, Kredit: %d)", ErrValidation, sumDebet, sumKredit)
	}

	// 2. Starta transaktionen (modernc.org/sqlite med _busy_timeout sköter wait-queue)
	tx, err := l.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	success := false
	var fileCreatedPath string
	defer func() {
		if !success {
			tx.Rollback()
			if fileCreatedPath != "" {
				if err := os.Remove(fileCreatedPath); err != nil && !os.IsNotExist(err) {
					log.Printf("Warning: Failed to remove orphan attachment file '%s': %v\n", fileCreatedPath, err)
				}
			}
		}
	}()

	res, filePath, err := l.postVerificationTx(tx, user, req)
	if err != nil {
		fileCreatedPath = filePath // Så att defer kan radera ifall vi skapade filen innan det smällde längre ner
		return nil, err
	}
	fileCreatedPath = filePath

	// 5. Commit
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	success = true

	// 6. Asynkron Auto-Seal (Lås alla utkast som är > 24h gamla)
	go func() {
		// Eftersom SealVerifications låser databasen under skrivningen, 
		// kan vi lägga in en liten delay så vi inte blockerar det just returnerade requestet.
		time.Sleep(100 * time.Millisecond)
		
		// Vi kollar om det finns NÅGON olåst verifikation som är äldre än 24 timmar.
		// Om ja, kör vi SealVerifications (som bygger kedjan för ALLA aktuella utkast).
		var count int
		err := l.db.QueryRow("SELECT COUNT(*) FROM verifications WHERE hash IS NULL AND created_at < datetime('now', '-24 hours')").Scan(&count)
		if err == nil && count > 0 {
			log.Printf("AutoSeal: Found %d unlocked verifications older than 24h. Sealing...", count)
			if _, err := l.SealVerifications("System Auto-Seal", true); err != nil {
				log.Printf("AutoSeal error: %v", err)
			} else {
				log.Printf("AutoSeal completed.")
			}
		}
	}()

	return res, nil
}

// postVerificationTx utför själva insert-logiken. Returnerar även eventuell skapad filsökväg för rollback-hantering.
func (l *Ledger) postVerificationTx(tx *sql.Tx, user string, req models.VerificationRequest) (*models.VerificationResult, string, error) {
	var fileCreatedPath string

	var attachmentHash *string
	var attachmentMime *string

	if req.AttachmentBase64 != "" {
		data, err := base64.StdEncoding.DecodeString(req.AttachmentBase64)
		if err != nil {
			return nil, "", fmt.Errorf("%w: ogiltig base64-kodning för bilagan", ErrValidation)
		}
		if len(data) > 10*1024*1024 {
			return nil, "", fmt.Errorf("%w: bilagan är för stor (max 10MB)", ErrValidation)
		}

		mimeType := http.DetectContentType(data)
		validMimes := map[string]string{
			"application/pdf": ".pdf",
			"image/png":       ".png",
			"image/jpeg":      ".jpg",
		}
		ext, valid := validMimes[mimeType]
		if !valid {
			return nil, "", fmt.Errorf("%w: ogiltig filtyp %s (måste vara PDF, PNG, eller JPEG)", ErrValidation, mimeType)
		}

		hashBytes := sha256.Sum256(data)
		hashStr := hex.EncodeToString(hashBytes[:])
		attachmentHash = &hashStr
		attachmentMime = &mimeType

		attachmentsDir := filepath.Join(l.workspacePath, "attachments")
		if err := os.MkdirAll(attachmentsDir, 0755); err != nil {
			return nil, "", fmt.Errorf("failed to create attachments directory: %w", err)
		}

		filePath := filepath.Join(attachmentsDir, hashStr+ext)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			if err := os.WriteFile(filePath, data, 0644); err != nil {
				return nil, "", fmt.Errorf("failed to write attachment: %w", err)
			}
			fileCreatedPath = filePath
		}
	}

	// 2.5 Pre-validera konton
	for _, row := range req.Rows {
		var exists int
		err := tx.QueryRow("SELECT 1 FROM accounts WHERE code = ?", row.Account).Scan(&exists)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, "", fmt.Errorf("%w: Ogiltigt konto '%s' (finns ej i kontoplanen)", ErrValidation, row.Account)
			}
			return nil, "", fmt.Errorf("failed to validate account %s: %w", row.Account, err)
		}
	}

	// 3. Lås-kontroll
	// 3a. Kontrollera Fiscal Year
	var fyLockedAt sql.NullString
	err := tx.QueryRow("SELECT locked_at FROM fiscal_years WHERE start_date <= ? AND end_date >= ?", req.Date, req.Date).Scan(&fyLockedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", ErrNoFiscalYear
		}
		return nil, "", fmt.Errorf("failed to query fiscal year: %w", err)
	}
	if fyLockedAt.Valid {
		return nil, "", ErrFiscalYearLocked
	}

	// 3b. Kontrollera Period Lock
	periodStr := req.Date[:7] // Ex: "2023-01"
	var plLockedAt string
	err = tx.QueryRow("SELECT locked_at FROM period_locks WHERE year_month = ?", periodStr).Scan(&plLockedAt)
	if err == nil {
		return nil, "", ErrPeriodLocked
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, "", fmt.Errorf("failed to query period locks: %w", err)
	}

	// 4. Databas-Insert
	// Huvudpost med RETURNING
	var vid int64
	var createdAt string
	vType := req.Type
	if vType == "" {
		vType = "NORMAL"
	}
	err = tx.QueryRow("INSERT INTO verifications (date, text, type, hash, attachment_hash, attachment_mime) VALUES (?, ?, ?, NULL, ?, ?) RETURNING id, created_at", req.Date, req.Text, vType, attachmentHash, attachmentMime).Scan(&vid, &createdAt)
	if err != nil {
		return nil, "", fmt.Errorf("failed to insert verification: %w", err)
	}

	// Rader
	for _, row := range req.Rows {
		_, err = tx.Exec("INSERT INTO verification_rows (verification_id, account, debet, kredit) VALUES (?, ?, ?, ?)",
			vid, row.Account, row.Debet, row.Kredit)
		if err != nil {
			return nil, "", fmt.Errorf("failed to insert row for account %s: %w", row.Account, err)
		}
	}

	// Audit Log
	auditText := fmt.Sprintf("Bokförde verifikation %d", vid)
	if err := l.logAuditTx(tx, user, "Post Verification", auditText); err != nil {
		return nil, "", err
	}

	// Returnera resultatet och filvägen (ifall anroparen behöver rulla tillbaka)
	return &models.VerificationResult{
		ID:        vid,
		CreatedAt: createdAt,
	}, fileCreatedPath, nil
}

// GetVerifications hämtar alla verifikationer med sina rader för GUI-presentation.
// Om yearID inte är nil filtreras verifikationerna för det specifika räkenskapsåret.
func (l *Ledger) GetVerifications(yearID *int64) ([]models.VerificationResponse, error) {
	// Denna metod läser från två tabeller och bygger en nästlad struktur.
	// För en lokal SQLite-databas är det enklast att köra en join och pussla ihop i Go.
	
	baseQuery := `
		SELECT 
			v.id, v.created_at, v.date, v.text, v.type, v.hash, v.storno_ref_id, v.attachment_hash, v.attachment_mime,
			EXISTS(SELECT 1 FROM verifications WHERE storno_ref_id = v.id) as is_stornoed,
			r.id, r.account, r.debet, r.kredit
		FROM verifications v
		LEFT JOIN verification_rows r ON v.id = r.verification_id
	`
	
	var rows *sql.Rows
	var err error

	if yearID != nil {
		var startDate, endDate string
		err := l.db.QueryRow("SELECT start_date, end_date FROM fiscal_years WHERE id = ?", *yearID).Scan(&startDate, &endDate)
		if err != nil {
			return nil, fmt.Errorf("failed to get fiscal year %d: %w", *yearID, err)
		}
		query := baseQuery + " WHERE v.date >= ? AND v.date <= ? ORDER BY v.id ASC, r.id ASC"
		rows, err = l.db.Query(query, startDate, endDate)
	} else {
		query := baseQuery + " ORDER BY v.id ASC, r.id ASC"
		rows, err = l.db.Query(query)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query verifications: %w", err)
	}
	defer rows.Close()

	var result []models.VerificationResponse
	var currentVerification *models.VerificationResponse

	for rows.Next() {
		var vID int64
		var vCreatedAt, vDate, vText, vType string
		var vHash, vAttachmentHash, vAttachmentMime sql.NullString
		var vStornoRefID sql.NullInt64
		var vIsStornoed bool
		
		var rID sql.NullInt64
		var rAccount sql.NullString
		var rDebet, rKredit sql.NullInt64

		err := rows.Scan(
			&vID, &vCreatedAt, &vDate, &vText, &vType, &vHash, &vStornoRefID, &vAttachmentHash, &vAttachmentMime, &vIsStornoed,
			&rID, &rAccount, &rDebet, &rKredit,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if currentVerification == nil || currentVerification.ID != vID {
			if currentVerification != nil {
				result = append(result, *currentVerification)
			}
			
			var hashPtr *string
			if vHash.Valid {
				hashStr := vHash.String
				hashPtr = &hashStr
			}

			var stornoRefPtr *int64
			if vStornoRefID.Valid {
				stornoRefVal := vStornoRefID.Int64
				stornoRefPtr = &stornoRefVal
			}

			var attHashPtr *string
			if vAttachmentHash.Valid {
				attHashStr := vAttachmentHash.String
				attHashPtr = &attHashStr
			}

			var attMimePtr *string
			if vAttachmentMime.Valid {
				attMimeStr := vAttachmentMime.String
				attMimePtr = &attMimeStr
			}

			currentVerification = &models.VerificationResponse{
				ID:             vID,
				CreatedAt:      vCreatedAt,
				Date:           vDate,
				Text:           vText,
				Type:           vType,
				Hash:           hashPtr,
				IsStornoed:     vIsStornoed,
				StornoRefID:    stornoRefPtr,
				AttachmentHash: attHashPtr,
				AttachmentMime: attMimePtr,
				Rows:           []models.RowResponse{},
			}
		}

		if rID.Valid {
			currentVerification.Rows = append(currentVerification.Rows, models.RowResponse{
				ID:      rID.Int64,
				Account: rAccount.String,
				Debet:   rDebet.Int64,
				Kredit:  rKredit.Int64,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating verification rows: %w", err)
	}

	if currentVerification != nil {
		result = append(result, *currentVerification)
	}

	// Om tom databas
	if result == nil {
		result = []models.VerificationResponse{}
	}

	return result, nil
}

// VoidDraftVerification makulerar ett utkast. Den tar bort raderna och sätter in nollrader,
// tar bort koppling till kvitto (men raderar ej filen) och ändrar texten.
// Avbryts om verifikationen redan är låst (hash != NULL).
func (l *Ledger) VoidDraftVerification(id int64, user string) error {
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

	var hash sql.NullString
	err = tx.QueryRow("SELECT hash FROM verifications WHERE id = ?", id).Scan(&hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("verification %d not found", id)
		}
		return fmt.Errorf("failed to fetch verification: %w", err)
	}

	if hash.Valid {
		return fmt.Errorf("kan inte makulera verifikation %d: den är redan WORM-låst", id)
	}

	// 1. Nollställ huvudposten
	_, err = tx.Exec("UPDATE verifications SET text = 'Makulerat utkast', attachment_hash = NULL, attachment_mime = NULL WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to update verification: %w", err)
	}

	// 2. Ta bort gamla rader
	_, err = tx.Exec("DELETE FROM verification_rows WHERE verification_id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete old rows: %w", err)
	}

	// 3. (Borttaget: Vi sätter inte in några nollrader, texten "Makulerat utkast" räcker)

	// 4. Audit Log
	auditText := fmt.Sprintf("Makulerade utkast %d", id)
	_, err = tx.Exec("INSERT INTO audit_log (user, action, details) VALUES (?, 'Void Draft', ?)", user, auditText)
	if err != nil {
		return fmt.Errorf("failed to insert audit log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	success = true
	return nil
}

// GetAttachmentInfo hämtar MIME-typ och absolut sökväg för en bilaga.
func (l *Ledger) GetAttachmentInfo(hash string) (string, string, error) {
	var mimeType string
	err := l.db.QueryRow("SELECT attachment_mime FROM verifications WHERE attachment_hash = ? LIMIT 1", hash).Scan(&mimeType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", fmt.Errorf("%w: attachment not found", ErrValidation)
		}
		return "", "", fmt.Errorf("failed to query attachment: %w", err)
	}

	validMimes := map[string]string{
		"application/pdf": ".pdf",
		"image/png":       ".png",
		"image/jpeg":      ".jpg",
	}
	ext := validMimes[mimeType]
	if ext == "" {
		return "", "", fmt.Errorf("%w: invalid mime type in database", ErrValidation)
	}

	filePath := filepath.Join(l.workspacePath, "attachments", hash+ext)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("%w: attachment file missing on disk", ErrValidation)
	}

	return mimeType, filePath, nil
}
