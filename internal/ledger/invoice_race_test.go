package ledger

import (
	"sync"
	"testing"

	"localledger/internal/models"
)

func TestRegisterPayment_ConcurrencyRace(t *testing.T) {
	workspace := setupTestWorkspace(t)
	l, err := OpenLedger(workspace, "v2.0.0")
	if err != nil {
		t.Fatalf("OpenLedger failed: %v", err)
	}
	defer l.Close()

	// 1. Skapa en faktura och posta den (så den blir 'bokförd')
	invID, err := l.CreateInvoice(models.Invoice{
		Date:             "2023-05-20",
		DueDate:          "2023-06-20",
		PaymentTermsDays: 30,
		CustomerName:     "Test Customer Concurrency",
		TotalAmount:      10000, // 100.00 kr
		TotalVat:         2000,  // 20.00 kr
		FiscalYearID:     1,
		Items: []models.InvoiceItem{
			{Description: "Test Item", Quantity: 100, PriceExVat: 8000, VatRate: 25},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create invoice: %v", err)
	}

	err = l.PostInvoice(invID, "TestUser")
	if err != nil {
		t.Fatalf("Failed to post invoice: %v", err)
	}

	// 2. Förbered 30 goroutines att anropa RegisterPayment parallellt
	numGoroutines := 30
	var wg sync.WaitGroup
	startChan := make(chan struct{})

	// Samla resultat
	results := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Vänta på startsignalen så att de startar så nära varandra som möjligt
			<-startChan
			results[idx] = l.RegisterPayment(invID, "2023-05-21", "TestUser")
		}(i)
	}

	// Släpp lös alla goroutines samtidigt!
	close(startChan)
	wg.Wait()

	// 3. Räkna framgångsrika vs misslyckade
	successCount := 0
	alreadyPaidCount := 0
	otherErrorCount := 0
	var lastOtherError error

	for _, resErr := range results {
		if resErr == nil {
			successCount++
		} else if resErr.Error() == "invoice is already paid" {
			alreadyPaidCount++
		} else {
			otherErrorCount++
			lastOtherError = resErr
		}
	}

	t.Logf("Concurrency results: Success=%d, AlreadyPaidErrors=%d, OtherErrors=%d", successCount, alreadyPaidCount, otherErrorCount)

	if successCount != 1 {
		t.Errorf("Expected exactly 1 goroutine to succeed, got %d", successCount)
	}

	if alreadyPaidCount != numGoroutines-1 {
		t.Errorf("Expected exactly %d 'already paid' errors, got %d", numGoroutines-1, alreadyPaidCount)
	}

	if otherErrorCount > 0 {
		t.Errorf("Encountered unexpected concurrency errors: %v", lastOtherError)
	}
}

func TestSettleInvoice_ConcurrencyRace(t *testing.T) {
	workspace := setupTestWorkspace(t)
	l, err := OpenLedger(workspace, "v2.0.0")
	if err != nil {
		t.Fatalf("OpenLedger failed: %v", err)
	}
	defer l.Close()

	// 1. Skapa en originalfaktura och posta den
	origID, err := l.CreateInvoice(models.Invoice{
		Date:             "2023-05-20",
		DueDate:          "2023-06-20",
		PaymentTermsDays: 30,
		CustomerName:     "Test Customer Settle",
		TotalAmount:      10000,
		TotalVat:         2000,
		FiscalYearID:     1,
		Items: []models.InvoiceItem{
			{Description: "Orig Item", Quantity: 100, PriceExVat: 8000, VatRate: 25},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create original invoice: %v", err)
	}
	err = l.PostInvoice(origID, "TestUser")
	if err != nil {
		t.Fatalf("Failed to post original invoice: %v", err)
	}

	// 2. Skapa en kreditfaktura
	creditID, err := l.CreateCreditInvoice(origID, "TestUser")
	if err != nil {
		t.Fatalf("Failed to create credit invoice: %v", err)
	}

	// Justera datumet till 2023 eftersom testdatabasen bara har räkenskapsår för 2023
	_, err = l.db.Exec("UPDATE invoices SET date = '2023-05-20', due_date = '2023-05-20' WHERE id = ?", creditID)
	if err != nil {
		t.Fatalf("Failed to adjust credit invoice date: %v", err)
	}

	// Posta kreditfakturan så den blir 'bokförd'
	err = l.PostInvoice(creditID, "TestUser")
	if err != nil {
		t.Fatalf("Failed to post credit invoice: %v", err)
	}

	// 3. Förbered 30 goroutines att anropa SettleInvoice parallellt
	numGoroutines := 30
	var wg sync.WaitGroup
	startChan := make(chan struct{})

	// Samla resultat
	results := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startChan
			results[idx] = l.SettleInvoice(creditID)
		}(i)
	}

	// Släpp lös!
	close(startChan)
	wg.Wait()

	// 4. Räkna
	successCount := 0
	alreadySettledCount := 0
	otherErrorCount := 0
	var lastOtherError error

	for _, resErr := range results {
		if resErr == nil {
			successCount++
		} else if resErr.Error() == "both invoices must be posted before they can be settled" {
			alreadySettledCount++
		} else {
			otherErrorCount++
			lastOtherError = resErr
		}
	}

	t.Logf("Settle Concurrency results: Success=%d, AlreadySettledErrors=%d, OtherErrors=%d", successCount, alreadySettledCount, otherErrorCount)

	if successCount != 1 {
		t.Errorf("Expected exactly 1 SettleInvoice goroutine to succeed, got %d", successCount)
	}

	if alreadySettledCount != numGoroutines-1 {
		t.Errorf("Expected exactly %d 'both invoices must be posted' errors, got %d", numGoroutines-1, alreadySettledCount)
	}

	if otherErrorCount > 0 {
		t.Errorf("Encountered unexpected Settle concurrency errors: %v", lastOtherError)
	}
}
