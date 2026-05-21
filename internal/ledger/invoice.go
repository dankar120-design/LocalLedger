package ledger

import (
	"database/sql"
	"fmt"
	"localledger/internal/models"
	"math"
	"time"
)

// createInvoiceTx utför fakturaskapande inuti en befintlig transaktion
func (l *Ledger) createInvoiceTx(tx *sql.Tx, inv models.Invoice) (int64, error) {
	// Ensure customer exists in customer register
	if inv.CustomerName != "" {
		if inv.CustomerID != nil && *inv.CustomerID > 0 {
			// Update customer details in case they edited them in draft
			_, err := tx.Exec(`UPDATE customers SET name = ?, orgnr = ?, address = ? WHERE id = ?`,
				inv.CustomerName, inv.CustomerOrgnr, inv.CustomerAddress, *inv.CustomerID)
			if err != nil {
				return 0, fmt.Errorf("failed to update customer details: %w", err)
			}
		} else {
			// Check if customer name already exists
			var existingID int64
			err := tx.QueryRow("SELECT id FROM customers WHERE name = ?", inv.CustomerName).Scan(&existingID)
			if err == sql.ErrNoRows {
				resCust, errCust := tx.Exec(`INSERT INTO customers (name, orgnr, address) VALUES (?, ?, ?)`,
					inv.CustomerName, inv.CustomerOrgnr, inv.CustomerAddress)
				if errCust != nil {
					return 0, fmt.Errorf("failed to insert customer: %w", errCust)
				}
				newID, errID := resCust.LastInsertId()
				if errID != nil {
					return 0, fmt.Errorf("failed to get new customer id: %w", errID)
				}
				inv.CustomerID = &newID
			} else if err == nil {
				inv.CustomerID = &existingID
				// Update details of existing customer
				_, err := tx.Exec(`UPDATE customers SET orgnr = ?, address = ? WHERE id = ?`,
					inv.CustomerOrgnr, inv.CustomerAddress, existingID)
				if err != nil {
					return 0, fmt.Errorf("failed to update existing customer details: %w", err)
				}
			} else {
				return 0, fmt.Errorf("failed to check existing customer: %w", err)
			}
		}
	}

	// WORM: Utkast får aldrig ha ett manuellt angivet fakturanummer. Det tilldelas vid PostInvoice.
	var invoiceNumber interface{} = nil

	res, err := tx.Exec(`
		INSERT INTO invoices (
			invoice_number, date, due_date, payment_terms_days, 
			customer_id, customer_name, customer_orgnr, customer_address, 
			total_amount, total_vat, status, credit_of, fiscal_year_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		invoiceNumber, inv.Date, inv.DueDate, inv.PaymentTermsDays,
		inv.CustomerID, inv.CustomerName, inv.CustomerOrgnr, inv.CustomerAddress,
		inv.TotalAmount, inv.TotalVat, "utkast", inv.CreditOf, inv.FiscalYearID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert invoice: %w", err)
	}

	invoiceID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

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
	return invoiceID, nil
}

// CreateInvoice sparar ett nytt fakturautkast
func (l *Ledger) CreateInvoice(inv models.Invoice) (int64, error) {
	tx, err := l.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	id, err := l.createInvoiceTx(tx, inv)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
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

	tx, err := l.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Acquire write lock immediately to prevent TOCTOU on SUM()
	_, err = tx.Exec("UPDATE invoices SET id = id WHERE id = ?", originalID)
	if err != nil {
		return 0, fmt.Errorf("failed to acquire concurrency lock: %w", err)
	}

	// Draft-hygiene: Prevent creating new drafts if already fully credited
	var sumExistingCredits int64
	err = tx.QueryRow("SELECT COALESCE(SUM(total_amount), 0) FROM invoices WHERE credit_of = ?", originalID).Scan(&sumExistingCredits)
	if err != nil {
		return 0, fmt.Errorf("could not sum existing credits: %w", err)
	}
	if int64(math.Abs(float64(sumExistingCredits))) >= origInv.TotalAmount {
		return 0, fmt.Errorf("invoice is already fully credited")
	}

	// Klonar fakturan med dagens datum
	now := time.Now().Format("2006-01-02")
	creditInv := models.Invoice{
		CreditOf:         &originalID,
		CustomerID:       origInv.CustomerID,
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

	id, err := l.createInvoiceTx(tx, creditInv)
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// GetInvoiceByID hämtar en specifik faktura och dess rader
func (l *Ledger) GetInvoiceByID(id int64) (models.Invoice, error) {
	var inv models.Invoice
	err := l.db.QueryRow(`
		SELECT id, invoice_number, date, due_date, payment_terms_days, 
		       customer_id, customer_name, customer_orgnr, customer_address, 
		       total_amount, total_vat, status, verification_id, credit_of, fiscal_year_id, created_at
		FROM invoices WHERE id = ?`, id).Scan(
		&inv.ID, &inv.InvoiceNumber, &inv.Date, &inv.DueDate, &inv.PaymentTermsDays,
		&inv.CustomerID, &inv.CustomerName, &inv.CustomerOrgnr, &inv.CustomerAddress,
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
		       customer_id, customer_name, customer_orgnr, customer_address, 
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
			&inv.CustomerID, &inv.CustomerName, &inv.CustomerOrgnr, &inv.CustomerAddress,
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
	existingInv, err := l.GetInvoiceByID(inv.ID)
	if err != nil {
		return fmt.Errorf("could not fetch existing invoice: %w", err)
	}
	if existingInv.VerificationID != nil {
		return fmt.Errorf("WORM VIOLATION: Cannot update a posted invoice")
	}

	if existingInv.CreditOf != nil {
		// Verify and lock down items
		origInv, err := l.GetInvoiceByID(*existingInv.CreditOf)
		if err != nil {
			return fmt.Errorf("could not fetch original invoice: %w", err)
		}

		var newTotalAmount int64 = 0
		var newTotalVat int64 = 0

		for _, item := range inv.Items {
			// Find matching original item by description
			var origItem *models.InvoiceItem
			for _, oi := range origInv.Items {
				if oi.Description == item.Description {
					origItem = &oi
					break
				}
			}
			if origItem == nil {
				return fmt.Errorf("cannot add new items to a credit invoice")
			}
			if item.PriceExVat != origItem.PriceExVat || item.VatRate != origItem.VatRate {
				return fmt.Errorf("cannot modify price or VAT rate on a credit invoice")
			}
			if item.Quantity > 0 {
				return fmt.Errorf("credit invoice quantity must be zero or negative")
			}
			if math.Abs(float64(item.Quantity)) > math.Abs(float64(origItem.Quantity)) {
				return fmt.Errorf("credit invoice quantity cannot exceed original quantity")
			}

			// Add to total
			lineExVatFloat := (float64(item.PriceExVat) * float64(item.Quantity)) / 100.0
			rowAmount := int64(math.Round(lineExVatFloat))
			rowVat := int64(math.Round(float64(rowAmount) * (float64(item.VatRate) / 100.0)))
			newTotalAmount += rowAmount + rowVat
			newTotalVat += rowVat
		}

		inv.TotalAmount = newTotalAmount
		inv.TotalVat = newTotalVat
	}

	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Ensure customer exists in customer register
	if inv.CustomerName != "" {
		if inv.CustomerID != nil && *inv.CustomerID > 0 {
			// Update customer details in case they edited them in draft
			_, err := tx.Exec(`UPDATE customers SET name = ?, orgnr = ?, address = ? WHERE id = ?`,
				inv.CustomerName, inv.CustomerOrgnr, inv.CustomerAddress, *inv.CustomerID)
			if err != nil {
				return fmt.Errorf("failed to update customer details: %w", err)
			}
		} else {
			// Check if customer name already exists
			var existingID int64
			err := tx.QueryRow("SELECT id FROM customers WHERE name = ?", inv.CustomerName).Scan(&existingID)
			if err == sql.ErrNoRows {
				resCust, errCust := tx.Exec(`INSERT INTO customers (name, orgnr, address) VALUES (?, ?, ?)`,
					inv.CustomerName, inv.CustomerOrgnr, inv.CustomerAddress)
				if errCust != nil {
					return fmt.Errorf("failed to insert customer: %w", errCust)
				}
				newID, errID := resCust.LastInsertId()
				if errID != nil {
					return fmt.Errorf("failed to get new customer id: %w", errID)
				}
				inv.CustomerID = &newID
			} else if err == nil {
				inv.CustomerID = &existingID
				// Update details of existing customer
				_, err := tx.Exec(`UPDATE customers SET orgnr = ?, address = ? WHERE id = ?`,
					inv.CustomerOrgnr, inv.CustomerAddress, existingID)
				if err != nil {
					return fmt.Errorf("failed to update existing customer details: %w", err)
				}
			} else {
				return fmt.Errorf("failed to check existing customer: %w", err)
			}
		}
	}

	// Determine InvoiceNumber value for DB. Since this is an update of a draft, we preserve any existing number or leave it nil.
	// But actually, WORM says drafts don't have numbers. So we don't update invoice_number here.
	_, err = tx.Exec(`
		UPDATE invoices SET 
			date = ?, due_date = ?, payment_terms_days = ?, 
			customer_id = ?, customer_name = ?, customer_orgnr = ?, customer_address = ?, 
			total_amount = ?, total_vat = ?, fiscal_year_id = ?
		WHERE id = ?`,
		inv.Date, inv.DueDate, inv.PaymentTermsDays,
		inv.CustomerID, inv.CustomerName, inv.CustomerOrgnr, inv.CustomerAddress,
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

	// Acquire exclusive write lock immediately to prevent TOCTOU on MAX(invoice_number)
	_, err = tx.Exec("UPDATE company_settings SET id = id WHERE id = 1")
	if err != nil {
		return fmt.Errorf("failed to acquire concurrency lock: %w", err)
	}

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
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Acquire exclusive write lock immediately
	_, err = tx.Exec("UPDATE company_settings SET id = id WHERE id = 1")
	if err != nil {
		return fmt.Errorf("failed to acquire concurrency lock: %w", err)
	}

	// Fetch invoice details inside the transaction to prevent TOCTOU
	var invStatus string
	var invVerificationID *int64
	var invTotalAmount int64
	var invInvoiceNumber *string
	var invCreditOf *int64

	err = tx.QueryRow(`
		SELECT status, verification_id, total_amount, invoice_number, credit_of 
		FROM invoices WHERE id = ?`, invoiceID).Scan(&invStatus, &invVerificationID, &invTotalAmount, &invInvoiceNumber, &invCreditOf)
	if err != nil {
		return err
	}

	if invStatus == "betald" {
		return fmt.Errorf("invoice is already paid")
	}
	if invVerificationID == nil {
		return fmt.Errorf("cannot pay an unposted draft invoice")
	}

	// 1. Calculate amount to pay (Adjust for partial credits if this is an original invoice)
	var sumCredits int64
	if invTotalAmount >= 0 && invCreditOf == nil {
		err = tx.QueryRow("SELECT COALESCE(SUM(total_amount), 0) FROM invoices WHERE credit_of = ? AND status IN ('bokförd', 'betald')", invoiceID).Scan(&sumCredits)
		if err != nil {
			return fmt.Errorf("failed to calculate sum of credits: %w", err)
		}
	}
	
	amount := invTotalAmount + sumCredits
	if amount == 0 {
		return fmt.Errorf("this invoice has been fully credited and cannot be paid via bank")
	}

	debitAccount := "1930"
	creditAccount := "1510"
	var invoiceNumStr string
	if invInvoiceNumber != nil {
		invoiceNumStr = *invInvoiceNumber
	}
	text := fmt.Sprintf("Inbetalning Faktura %s", invoiceNumStr)

	if amount < 0 {
		amount = -amount
		debitAccount = "1510"
		creditAccount = "1930"
		text = fmt.Sprintf("Utbetalning Kreditfaktura %s", invoiceNumStr)
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
	tx, err := l.db.Begin()
	if err != nil { return err }
	defer tx.Rollback()

	// Acquire exclusive write lock immediately
	_, err = tx.Exec("UPDATE company_settings SET id = id WHERE id = 1")
	if err != nil {
		return fmt.Errorf("failed to acquire concurrency lock: %w", err)
	}

	// Fetch credit invoice details
	var creditStatus string
	var creditOf *int64
	err = tx.QueryRow("SELECT status, credit_of FROM invoices WHERE id = ?", creditID).Scan(&creditStatus, &creditOf)
	if err != nil { return err }

	if creditOf == nil {
		return fmt.Errorf("not a credit invoice")
	}

	originalID := *creditOf

	// Fetch original invoice details
	var origStatus string
	var origTotalAmount int64
	err = tx.QueryRow("SELECT status, total_amount FROM invoices WHERE id = ?", originalID).Scan(&origStatus, &origTotalAmount)
	if err != nil { return err }

	if creditStatus != "bokförd" || (origStatus != "bokförd" && origStatus != "betald") {
		return fmt.Errorf("both invoices must be posted before they can be settled")
	}

	var sumCredits int64
	err = tx.QueryRow("SELECT COALESCE(SUM(total_amount), 0) FROM invoices WHERE credit_of = ? AND status IN ('bokförd', 'betald')", originalID).Scan(&sumCredits)
	if err != nil { return err }

	_, err = tx.Exec("UPDATE invoices SET status = 'betald' WHERE id = ?", creditID)
	if err != nil {
		return fmt.Errorf("failed to settle credit invoice: %w", err)
	}

	if int64(math.Abs(float64(sumCredits))) >= origTotalAmount {
		_, err = tx.Exec("UPDATE invoices SET status = 'betald' WHERE id = ?", originalID)
		if err != nil {
			return fmt.Errorf("failed to settle original invoice: %w", err)
		}
	}

	return tx.Commit()
}
