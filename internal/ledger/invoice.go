package ledger

import (
	"fmt"
	"localledger/internal/models"
	"math"
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
		// Quantity is in hundredths (150 = 1.5)
		lineExVat := (item.PriceExVat * int64(item.Quantity)) / 100
		
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
		Rows: rows,
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

	// 1. Create Payment Verification
	req := models.VerificationRequest{
		Date: date,
		Text: fmt.Sprintf("Inbetalning Faktura %s", *inv.InvoiceNumber),
		Type: "NORMAL",
		Rows: []models.RowRequest{
			{Account: "1930", Debet: inv.TotalAmount, Kredit: 0},
			{Account: "1510", Debet: 0, Kredit: inv.TotalAmount},
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
