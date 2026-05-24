package ledger_test

import (
	"os"
	"testing"

	"localledger/internal/ledger"
	"localledger/internal/models"
)

func TestMatchBankTransactions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rec_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	err = ledger.InitWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("InitWorkspace failed: %v", err)
	}

	l, err := ledger.OpenLedger(tmpDir, "v3.0.0")
	if err != nil {
		t.Fatalf("OpenLedger failed: %v", err)
	}
	defer l.Close()

	// 1. Skapa en faktura och bokför den (ger status = 'bokförd')
	invoice := models.Invoice{
		Date:             "2026-05-01",
		DueDate:          "2026-05-31",
		PaymentTermsDays: 30,
		CustomerName:     "TestKund AB",
		TotalAmount:      100000, // 1000.00 kr totalt inkl moms
		TotalVat:         20000,
		FiscalYearID:     1,
		Items: []models.InvoiceItem{
			{
				Description: "Konsulttjänster",
				Quantity:    100, // 1.0 enhet
				PriceExVat:  80000,
				VatRate:     25,
			},
		},
	}
	
	invID, err := l.CreateInvoice(invoice)
	if err != nil {
		t.Fatalf("CreateInvoice failed: %v", err)
	}

	err = l.PostInvoice(invID, "Test User")
	if err != nil {
		t.Fatalf("PostInvoice failed: %v", err)
	}

	// 2. Skapa en matchande banktransaktion (verifikation med 1930 debet på 1000 kr)
	// Först en med felaktigt belopp
	_, err = l.PostVerification("Test User", models.VerificationRequest{
		Date: "2026-05-28",
		Text: "Felaktigt belopp",
		Rows: []models.RowRequest{
			{Account: "1930", Debet: 50000, Kredit: 0},
			{Account: "3001", Debet: 0, Kredit: 50000},
		},
	})
	if err != nil {
		t.Fatalf("PostVerification failed: %v", err)
	}

	// Sedan en med rätt belopp och nästan rätt datum (±3 dagar från förfallodatum 2026-05-31)
	depVer, err := l.PostVerification("Test User", models.VerificationRequest{
		Date: "2026-05-30",
		Text: "Banköverföring Faktura 1000",
		Rows: []models.RowRequest{
			{Account: "1930", Debet: 100000, Kredit: 0}, // 1000.00 kr
			{Account: "3001", Debet: 0, Kredit: 100000},
		},
	})
	if err != nil {
		t.Fatalf("PostVerification failed: %v", err)
	}

	// Kör matchningen!
	matches, err := l.MatchBankTransactions()
	if err != nil {
		t.Fatalf("MatchBankTransactions failed: %v", err)
	}

	if len(matches) != 1 {
		t.Fatalf("Expected exactly 1 match, got %d", len(matches))
	}

	match := matches[0]
	if match.Invoice.ID != invID {
		t.Errorf("Expected match for invoice ID %d, got %d", invID, match.Invoice.ID)
	}
	if match.Verification.ID != depVer.ID {
		t.Errorf("Expected match with verification ID %d, got %d", depVer.ID, match.Verification.ID)
	}
	if match.Confidence != "HIGH" {
		t.Errorf("Expected HIGH confidence due to invoice number reference, got %s", match.Confidence)
	}
}
