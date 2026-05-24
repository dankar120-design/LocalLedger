package ledger

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"localledger/internal/models"
)

// MatchBankTransactions söker efter matchningar mellan obetalda fakturor (1510) och banktransaktioner (1930 debet).
func (l *Ledger) MatchBankTransactions() ([]models.ReconciliationMatch, error) {
	// 1. Hämta alla obetalda, bokförda fakturor
	invoicesRows, err := l.db.Query(`
		SELECT id, invoice_number, date, due_date, payment_terms_days, 
		       customer_id, customer_name, customer_orgnr, customer_address, 
		       total_amount, total_vat, status, verification_id, credit_of, fiscal_year_id, created_at
		FROM invoices 
		WHERE status = 'bokförd'
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query unpaid invoices: %w", err)
	}
	defer invoicesRows.Close()

	var unpaidInvoices []models.Invoice
	for invoicesRows.Next() {
		var inv models.Invoice
		err := invoicesRows.Scan(
			&inv.ID, &inv.InvoiceNumber, &inv.Date, &inv.DueDate, &inv.PaymentTermsDays,
			&inv.CustomerID, &inv.CustomerName, &inv.CustomerOrgnr, &inv.CustomerAddress,
			&inv.TotalAmount, &inv.TotalVat, &inv.Status, &inv.VerificationID, &inv.CreditOf, &inv.FiscalYearID, &inv.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan invoice: %w", err)
		}
		unpaidInvoices = append(unpaidInvoices, inv)
	}

	if err = invoicesRows.Err(); err != nil {
		return nil, fmt.Errorf("invoices rows iteration error: %w", err)
	}

	// 2. Hämta alla banktransaktioner (verifikationer med insättning på konto 1930) via en optimerad SQL-fråga
	// Detta undviker att läsa in alla verifikationer i systemet (eliminerar O(N) prestandabomb).
	rows, err := l.db.Query(`
		SELECT 
			v.id, v.created_at, v.date, v.text, v.type, v.hash, v.storno_ref_id, v.attachment_hash, v.attachment_mime,
			EXISTS(SELECT 1 FROM verifications WHERE storno_ref_id = v.id) as is_stornoed,
			r.id, r.account, r.debet, r.kredit
		FROM verifications v
		LEFT JOIN verification_rows r ON v.id = r.verification_id
		WHERE v.id IN (SELECT DISTINCT verification_id FROM verification_rows WHERE account = '1930' AND debet > 0)
		  AND v.text NOT LIKE 'Inbetalning Faktura%'
		ORDER BY v.id ASC, r.id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query bank verifications: %w", err)
	}
	defer rows.Close()

	var bankDeposits []models.VerificationResponse
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
			return nil, fmt.Errorf("failed to scan verification row: %w", err)
		}

		if currentVerification == nil || currentVerification.ID != vID {
			if currentVerification != nil {
				bankDeposits = append(bankDeposits, *currentVerification)
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

			var attHashPtr, attMimePtr *string
			if vAttachmentHash.Valid {
				attHashStr := vAttachmentHash.String
				attHashPtr = &attHashStr
			}
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
	if currentVerification != nil {
		bankDeposits = append(bankDeposits, *currentVerification)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("bank verifications rows iteration error: %w", err)
	}

	var matches []models.ReconciliationMatch
	
	// Använd en map för att flagga/konsumera transaktioner för att undvika matchningskollisioner.
	usedDeposits := make(map[int64]bool)

	// 3. Matchningsalgoritm
	for _, inv := range unpaidInvoices {
		var bestMatch *models.VerificationResponse
		var maxScore = 0
		var bestReason = ""
		var bestConfidence = "LOW"

		invNum := ""
		if inv.InvoiceNumber != nil {
			invNum = *inv.InvoiceNumber
		}

		invDate, err1 := time.Parse("2006-01-02", inv.Date)
		invDueDate, err2 := time.Parse("2006-01-02", inv.DueDate)

		for _, dep := range bankDeposits {
			// Hoppa över redan använda transaktioner
			if usedDeposits[dep.ID] {
				continue
			}

			var depAmount int64
			for _, row := range dep.Rows {
				if row.Account == "1930" {
					depAmount = row.Debet
					break
				}
			}

			// A. Beloppsmatchning
			if depAmount != inv.TotalAmount {
				continue
			}

			score := 0
			reasonParts := []string{"Exakt beloppsmatchning"}

			// B. Text/OCR/Fakturanummer Matchning
			textLower := strings.ToLower(dep.Text)
			hasOCR := false
			if invNum != "" && (strings.Contains(textLower, invNum) || strings.Contains(textLower, "ocr")) {
				score += 50
				hasOCR = true
				reasonParts = append(reasonParts, fmt.Sprintf("Fakturanummer '%s' funnet i texten", invNum))
			}

			// C. Datum Matchning
			depDate, err3 := time.Parse("2006-01-02", dep.Date)
			if err1 == nil && err2 == nil && err3 == nil {
				daysToDue := math.Abs(depDate.Sub(invDueDate).Hours() / 24)
				daysToInv := math.Abs(depDate.Sub(invDate).Hours() / 24)

				if daysToDue <= 3 {
					score += 30
					reasonParts = append(reasonParts, fmt.Sprintf("Nära förfallodatum (%.0f dagar)", daysToDue))
				} else if daysToInv <= 3 {
					score += 20
					reasonParts = append(reasonParts, fmt.Sprintf("Nära fakturadatum (%.0f dagar)", daysToInv))
				} else if depDate.After(invDate) && depDate.Before(invDueDate.AddDate(0, 0, 30)) {
					score += 10
					reasonParts = append(reasonParts, "Betalningsdatum inom rimligt intervall")
				}
			}

			// Spara bästa matchningen
			if score > maxScore {
				maxScore = score
				depCopy := dep
				bestMatch = &depCopy
				bestReason = strings.Join(reasonParts, ", ")

				if hasOCR && score >= 70 {
					bestConfidence = "HIGH"
				} else if score >= 30 {
					bestConfidence = "MEDIUM"
				} else {
					bestConfidence = "LOW"
				}
			}
		}

		if bestMatch != nil {
			usedDeposits[bestMatch.ID] = true
			matches = append(matches, models.ReconciliationMatch{
				Invoice:      inv,
				Verification: *bestMatch,
				Confidence:   bestConfidence,
				Reason:       bestReason,
			})
		}
	}

	return matches, nil
}
