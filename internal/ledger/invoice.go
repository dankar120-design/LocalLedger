package ledger

import (
	"fmt"
	"localledger/internal/models"
	"math"
	"time"
)

// CreateInvoice sparar ett nytt fakturautkast
func (l *Ledger) CreateInvoice(inv models.Invoice) (int64, error) {
	// WORM check is not needed for Create, since it's a new draft.
	
	// Start transaction
	tx, err := l.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Determine InvoiceNumber value for DB (nil if empty)
	var invoiceNumber interface{} = nil
	if inv.InvoiceNumber != nil && *inv.InvoiceNumber != "" {
		invoiceNumber = *inv.InvoiceNumber
	}

	// Insert Invoice
	res, err := tx.Exec(`
		INSERT INTO invoices (
			invoice_number, date, due_date, payment_terms_days, 
			customer_name, customer_orgnr, customer_address, 
			total_amount, total_vat, status, fiscal_year_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		invoiceNumber, inv.Date, inv.DueDate, inv.PaymentTermsDays,
		inv.CustomerName, inv.CustomerOrgnr, inv.CustomerAddress,
		inv.TotalAmount, inv.TotalVat, "utkast", inv.FiscalYearID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert invoice: %w", err)
	}

	invoiceID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Insert Items
	for _, item := range inv.Items {
		_, err := tx.Exec(`
			INSERT INTO invoice_items (invoice_id, description, quantity, price_ex_vat, vat_rate)
			VALUES (?, ?, ?, ?, ?)`,
			invoiceID, item.Description, item.Quantity, item.PriceExVat, item.VatRate,
		)
		if err != nil {
			return 0, fmt.Errorf("failed to insert invoice item: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return invoiceID, nil
}

// CreateCreditInvoice skapar en kreditfaktura baserad på ett original
func (l *Ledger) CreateCreditInvoice(originalID int64, user string) (int64, error) {
	origInv, err := l.GetInvoiceByID(originalID)
	if err != nil {
		return 0, err
	}
	if origInv.CreditOf != nil {
		return 0, fmt.Errorf("cannot credit a credit invoice")
	}
	if origInv.Status == "utkast" {
		return 0, fmt.Errorf("cannot credit an unposted invoice")
	}

	// Klonar fakturan med dagens datum
	now := time.Now().Format("2006-01-02")
	creditInv := models.Invoice{
		CreditOf:         &originalID,
		Date:             now,
		DueDate:          now, // Kreditfakturor saknar förfallodatum
		PaymentTermsDays: 0,
		CustomerName:     origInv.CustomerName,
		CustomerOrgnr:    origInv.CustomerOrgnr,
		CustomerAddress:  origInv.CustomerAddress,
		TotalAmount:      -origInv.TotalAmount,
		TotalVat:         -origInv.TotalVat,
		Status:           "utkast",
		FiscalYearID:     origInv.FiscalYearID,
	}

	for _, item := range origInv.Items {
		creditInv.Items = append(creditInv.Items, models.InvoiceItem{
			Description: item.Description,
			Quantity:    -item.Quantity, // Negativ kvantitet
			PriceExVat:  item.PriceExVat,
			VatRate:     item.VatRate,
		})
	}

	return l.CreateInvoice(creditInv)
}

// GetInvoiceByID hämtar en specifik faktura och dess rader
func (l *Ledger) GetInvoiceByID(id int64) (models.Invoice, error) {
	var inv models.Invoice
	err := l.db.QueryRow(`
		SELECT id, invoice_number, date, due_date, payment_terms_days, 
		       customer_name, customer_orgnr, customer_address, 
		       total_amount, total_vat, status, verification_id, credit_of, fiscal_year_id, created_at
		FROM invoices WHERE id = ?`, id).Scan(
		&inv.ID, &inv.InvoiceNumber, &inv.Date, &inv.DueDate, &inv.PaymentTermsDays,
		&inv.CustomerName, &inv.CustomerOrgnr, &inv.CustomerAddress,
		&inv.TotalAmount, &inv.TotalVat, &inv.Status, &inv.VerificationID, &inv.CreditOf, &inv.FiscalYearID, &inv.CreatedAt,
	)
	if err != nil {
		return inv, err
	}

	rows, err := l.db.Query(`SELECT id, description, quantity, price_ex_vat, vat_rate FROM invoice_items WHERE invoice_id = ?`, id)
	if err != nil {
		return inv, err
	}
	defer rows.Close()

	for rows.Next() {
		var item models.InvoiceItem
		if err := rows.Scan(&item.ID, &item.Description, &item.Quantity, &item.PriceExVat, &item.VatRate); err != nil {
			return inv, err
		}
		item.InvoiceID = id
		inv.Items = append(inv.Items, item)
	}

	return inv, nil
}

// GetInvoices hämtar alla fakturor för ett visst räkenskapsår
func (l *Ledger) GetInvoices(fiscalYearID int64) ([]models.Invoice, error) {
	rows, err := l.db.Query(`
		SELECT id, invoice_number, date, due_date, payment_terms_days, 
		       customer_name, customer_orgnr, customer_address, 
		       total_amount, total_vat, status, verification_id, credit_of, fiscal_year_id, created_at
		FROM invoices WHERE fiscal_year_id = ? ORDER BY id DESC`, fiscalYearID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invoices []models.Invoice
	for rows.Next() {
		var inv models.Invoice
		if err := rows.Scan(
			&inv.ID, &inv.InvoiceNumber, &inv.Date, &inv.DueDate, &inv.PaymentTermsDays,
			&inv.CustomerName, &inv.CustomerOrgnr, &inv.CustomerAddress,
			&inv.TotalAmount, &inv.TotalVat, &inv.Status, &inv.VerificationID, &inv.CreditOf, &inv.FiscalYearID, &inv.CreatedAt,
		); err != nil {
			return nil, err
		}
		invoices = append(invoices, inv)
	}
	return invoices, nil
}

// UpdateInvoice uppdaterar ett fakturautkast (helt och hållet). Avvisas om den är låst (verification_id != NULL)
func (l *Ledger) UpdateInvoice(inv models.Invoice) error {
	// 1. WORM Check: Is it locked?
	var verID *int64
	err := l.db.QueryRow("SELECT verification_id FROM invoices WHERE id = ?", inv.ID).Scan(&verID)
	if err != nil {
		return fmt.Errorf("could not check invoice lock status: %w", err)
	}
	if verID != nil {
		return fmt.Errorf("WORM VIOLATION: Cannot update a posted invoice")
	}

	if inv.CreditOf != nil {
		if inv.TotalAmount > 0 {
			return fmt.Errorf("a credit invoice cannot have a positive total amount")
		}
	}

	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Determine InvoiceNumber value for DB (nil if empty)
	var invoiceNumber interface{} = nil
	if inv.InvoiceNumber != nil && *inv.InvoiceNumber != "" {
		invoiceNumber = *inv.InvoiceNumber
	}

	_, err = tx.Exec(`
		UPDATE invoices SET 
			invoice_number = ?, date = ?, due_date = ?, payment_terms_days = ?, 
			customer_name = ?, customer_orgnr = ?, customer_address = ?, 
			total_amount = ?, total_vat = ?, fiscal_year_id = ?
		WHERE id = ?`,
		invoiceNumber, inv.Date, inv.DueDate, inv.PaymentTermsDays,
		inv.CustomerName, inv.CustomerOrgnr, inv.CustomerAddress,
		inv.TotalAmount, inv.TotalVat, inv.FiscalYearID, inv.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update invoice header: %w", err)
	}

	// Delete old items and insert new ones
	if _, err := tx.Exec("DELETE FROM invoice_items WHERE invoice_id = ?", inv.ID); err != nil {
		return fmt.Errorf("failed to delete old invoice items: %w", err)
	}

	for _, item := range inv.Items {
		_, err := tx.Exec(`
			INSERT INTO invoice_items (invoice_id, description, quantity, price_ex_vat, vat_rate)
			VALUES (?, ?, ?, ?, ?)`,
			inv.ID, item.Description, item.Quantity, item.PriceExVat, item.VatRate,
		)
		if err != nil {
			return fmt.Errorf("failed to insert new invoice item: %w", err)
		}
	}

	return tx.Commit()
}

// DeleteInvoice tar bort ett fakturautkast. Avvisas om den är låst.
func (l *Ledger) DeleteInvoice(id int64) error {
	// 1. WORM Check: Is it locked?
	var verID *int64
	err := l.db.QueryRow("SELECT verification_id FROM invoices WHERE id = ?", id).Scan(&verID)
	if err != nil {
		return fmt.Errorf("could not check invoice lock status: %w", err)
	}
	if verID != nil {
		return fmt.Errorf("WORM VIOLATION: Cannot delete a posted invoice")
	}

	// Because of ON DELETE CASCADE on invoice_items, we only need to delete the invoice
	_, err = l.db.Exec("DELETE FROM invoices WHERE id = ?", id)
	return err
}

// PostInvoice låser en faktura, ger den ett fakturanummer och skapar en verifikation i huvudboken (Zero Double-Entry)
func (l *Ledger) PostInvoice(invoiceID int64, user string) error {
	inv, err := l.GetInvoiceByID(invoiceID)
	if err != nil {
		return err
	}
	if inv.VerificationID != nil {
		return fmt.Errorf("invoice is already posted")
	}

	if inv.CreditOf != nil {
		if inv.TotalAmount > 0 {
			return fmt.Errorf("a credit invoice cannot have a positive total amount")
		}
		
		origInv, err := l.GetInvoiceByID(*inv.CreditOf)
		if err != nil {
			return fmt.Errorf("could not fetch original invoice: %w", err)
		}
		
		var sumPostedCredits int64
		err = l.db.QueryRow("SELECT COALESCE(SUM(total_amount), 0) FROM invoices WHERE credit_of = ? AND status IN ('bokförd', 'betald')", *inv.CreditOf).Scan(&sumPostedCredits)
		if err != nil {
			return fmt.Errorf("could not sum existing credits: %w", err)
		}
		
		if int64(math.Abs(float64(sumPostedCredits + inv.TotalAmount))) > origInv.TotalAmount {
			return fmt.Errorf("cannot credit more than the original invoice amount (Total available to credit: %.2f)", float64(origInv.TotalAmount - int64(math.Abs(float64(sumPostedCredits)))) / 100.0)
		}
	}

	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Determine next invoice number
	var nextNum int
	err = tx.QueryRow("SELECT COALESCE(MAX(CAST(invoice_number AS INTEGER)), 0) FROM invoices WHERE invoice_number IS NOT NULL AND invoice_number != ''").Scan(&nextNum)
	if err != nil || nextNum == 0 {
		// Fallback to settings
		err = tx.QueryRow("SELECT invoice_start_number FROM company_settings WHERE id = 1").Scan(&nextNum)
		if err != nil {
			nextNum = 1000 // safe fallback
		}
	} else {
		nextNum++
	}
	newInvoiceNumber := fmt.Sprintf("%d", nextNum)

	// 2. Calculate Sales and VAT mappings
	salesByVat := make(map[int]int64)
	vatByVat := make(map[int]int64)

	for _, item := range inv.Items {
		// Quantity is in hundredths (150 = 1.5). Calculate with floats to prevent truncation errors.
		lineExVatFloat := (float64(item.PriceExVat) * float64(item.Quantity)) / 100.0
		lineExVat := int64(math.Round(lineExVatFloat))
		
		lineVatFloat := float64(lineExVat) * float64(item.VatRate) / 100.0
		lineVat := int64(math.Round(lineVatFloat))

		salesByVat[item.VatRate] += lineExVat
		vatByVat[item.VatRate] += lineVat
	}

	var totalSales int64
	var totalVat int64
	var rows []models.RowRequest

	// Bas-kontoplan mapping
	salesAccounts := map[int]string{25: "3001", 12: "3002", 6: "3003", 0: "3004"}
	vatAccounts := map[int]string{25: "2611", 12: "2621", 6: "2631"}

	// Sortera momssatserna för att garantera deterministisk ordning (25, 12, 6, 0)
	rates := []int{25, 12, 6, 0}
	
	for _, rate := range rates {
		if amount := salesByVat[rate]; amount != 0 {
			totalSales += amount
			acc, ok := salesAccounts[rate]
			if !ok { acc = "3001" }
			rows = append(rows, models.RowRequest{Account: acc, Debet: 0, Kredit: amount})
		}
		if amount := vatByVat[rate]; amount != 0 {
			totalVat += amount
			acc, ok := vatAccounts[rate]
			if !ok { acc = "2611" }
			rows = append(rows, models.RowRequest{Account: acc, Debet: 0, Kredit: amount})
		}
	}

	// 1510 Kundfordringar (läggs alltid sist för läsbarhet, eller först. Vi lägger den sist)
	totalDebit := totalSales + totalVat
	if totalDebit == 0 {
		return fmt.Errorf("cannot post an invoice with 0 total amount") // Skydd mot 0-kronors krasch
	}
	
	rows = append([]models.RowRequest{{Account: "1510", Debet: totalDebit, Kredit: 0}}, rows...)

	var flippedRows []models.RowRequest
	for _, r := range rows {
		d := r.Debet
		k := r.Kredit
		if d < 0 {
			k += -d
			d = 0
		}
		if k < 0 {
			d += -k
			k = 0
		}
		flippedRows = append(flippedRows, models.RowRequest{Account: r.Account, Debet: d, Kredit: k})
	}

	// 3. Generate PDF (WORM Attachment)
	settings, err := l.GetSettings()
	if err != nil {
		return fmt.Errorf("failed to get company settings for pdf: %w", err)
	}

	pdfBase64, err := GenerateInvoicePDF(inv, settings, newInvoiceNumber)
	if err != nil {
		return fmt.Errorf("failed to generate pdf: %w", err)
	}

	// 4. Create Verification
	req := models.VerificationRequest{
		Date: inv.Date,
		Text: fmt.Sprintf("Faktura %s - %s", newInvoiceNumber, inv.CustomerName),
		Type: "NORMAL",
		Rows: flippedRows,
		AttachmentBase64: pdfBase64,
	}

	verRes, _, err := l.postVerificationTx(tx, user, req)
	if err != nil {
		return fmt.Errorf("failed to create verification: %w", err)
	}

	// 4. Update Invoice
	_, err = tx.Exec(`
		UPDATE invoices 
		SET invoice_number = ?, status = 'bokförd', verification_id = ?, total_amount = ?, total_vat = ? 
		WHERE id = ?`, 
		newInvoiceNumber, verRes.ID, totalDebit, totalVat, invoiceID)
	if err != nil {
		return fmt.Errorf("failed to lock invoice: %w", err)
	}

	return tx.Commit()
}

// RegisterPayment bokför en betalning av en faktura och sätter status till 'betald' (Anti Split-Brain)
func (l *Ledger) RegisterPayment(invoiceID int64, date string, user string) error {
	inv, err := l.GetInvoiceByID(invoiceID)
	if err != nil {
		return err
	}
	if inv.Status == "betald" {
		return fmt.Errorf("invoice is already paid")
	}
	if inv.VerificationID == nil {
		return fmt.Errorf("cannot pay an unposted draft invoice")
	}

	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Calculate amount to pay (Adjust for partial credits if this is an original invoice)
	var sumCredits int64
	if inv.TotalAmount >= 0 && inv.CreditOf == nil {
		err = l.db.QueryRow("SELECT COALESCE(SUM(total_amount), 0) FROM invoices WHERE credit_of = ? AND status IN ('bokförd', 'betald')", invoiceID).Scan(&sumCredits)
		if err != nil {
			return fmt.Errorf("failed to calculate sum of credits: %w", err)
		}
	}
	
	amount := inv.TotalAmount + sumCredits
	if amount == 0 {
		return fmt.Errorf("this invoice has been fully credited and cannot be paid via bank")
	}

	debitAccount := "1930"
	creditAccount := "1510"
	text := fmt.Sprintf("Inbetalning Faktura %s", *inv.InvoiceNumber)

	if amount < 0 {
		amount = -amount
		debitAccount = "1510"
		creditAccount = "1930"
		text = fmt.Sprintf("Utbetalning Kreditfaktura %s", *inv.InvoiceNumber)
	}

	req := models.VerificationRequest{
		Date: date,
		Text: text,
		Type: "NORMAL",
		Rows: []models.RowRequest{
			{Account: debitAccount, Debet: amount, Kredit: 0},
			{Account: creditAccount, Debet: 0, Kredit: amount},
		},
	}

	_, _, err = l.postVerificationTx(tx, user, req)
	if err != nil {
		return fmt.Errorf("failed to create payment verification: %w", err)
	}

	// 2. Update Invoice Status
	_, err = tx.Exec("UPDATE invoices SET status = 'betald' WHERE id = ?", invoiceID)
	if err != nil {
		return fmt.Errorf("failed to update invoice status: %w", err)
	}

	return tx.Commit()
}

// SettleInvoice kvittar en kreditfaktura mot sin originalfaktura och stänger båda.
func (l *Ledger) SettleInvoice(creditID int64) error {
	creditInv, err := l.GetInvoiceByID(creditID)
	if err != nil { return err }
	if creditInv.CreditOf == nil {
		return fmt.Errorf("not a credit invoice")
	}

	originalID := *creditInv.CreditOf
	origInv, err := l.GetInvoiceByID(originalID)
	if err != nil { return err }

	if creditInv.Status != "bokförd" || origInv.Status != "bokförd" {
		return fmt.Errorf("both invoices must be posted before they can be settled")
	}

	tx, err := l.db.Begin()
	if err != nil { return err }
	defer tx.Rollback()

	var sumCredits int64
	err = tx.QueryRow("SELECT COALESCE(SUM(total_amount), 0) FROM invoices WHERE credit_of = ? AND status IN ('bokförd', 'betald')", originalID).Scan(&sumCredits)
	if err != nil { return err }

	_, err = tx.Exec("UPDATE invoices SET status = 'betald' WHERE id = ?", creditID)
	if err != nil {
		return fmt.Errorf("failed to settle credit invoice: %w", err)
	}

	if int64(math.Abs(float64(sumCredits))) >= origInv.TotalAmount {
		_, err = tx.Exec("UPDATE invoices SET status = 'betald' WHERE id = ?", originalID)
		if err != nil {
			return fmt.Errorf("failed to settle original invoice: %w", err)
		}
	}

	return tx.Commit()
}
