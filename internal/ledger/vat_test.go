package ledger

import (
	"database/sql"
	"testing"

	"localledger/internal/models"
	_ "modernc.org/sqlite"
)

func setupVatTestDB(t *testing.T) *Ledger {
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	db.SetMaxOpenConns(1)

	if err := runMigrations(db); err != nil {
		t.Fatalf("runMigrations failed: %v", err)
	}

	// Create fiscal year 2026 (migrations may have already created id 1)
	_, err = db.Exec("INSERT INTO fiscal_years (start_date, end_date) VALUES ('2026-01-01', '2026-12-31')")
	if err != nil {
		t.Fatalf("Failed to insert fiscal year: %v", err)
	}

	return &Ledger{db: db}
}

func TestGetVatReport_Standard(t *testing.T) {
	l := setupVatTestDB(t)

	// Inrikes försäljning (3001 = 25%, 3002 = 12%)
	// V1: Sälj varor 1000kr ex moms (25%). Total 1250kr
	// V2: Sälj varor 500kr ex moms (12%). Total 560kr
	
	_, err := l.PostVerification("System", models.VerificationRequest{
		Date: "2026-01-10",
		Text: "Försäljning 25%",
		Rows: []models.RowRequest{
			{Account: "1930", Debet: 1250},
			{Account: "2611", Kredit: 250}, // Ruta 10
			{Account: "3001", Kredit: 1000}, // Ruta 05
		},
	})
	if err != nil {
		t.Fatalf("V1 failed: %v", err)
	}

	_, err = l.PostVerification("System", models.VerificationRequest{
		Date: "2026-01-15",
		Text: "Försäljning 12%",
		Rows: []models.RowRequest{
			{Account: "1930", Debet: 560},
			{Account: "2620", Kredit: 60}, // Ruta 11
			{Account: "3002", Kredit: 500}, // Ruta 06
		},
	})
	if err != nil {
		t.Fatalf("V2 failed: %v", err)
	}

	report, err := l.GetVatReport("2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("GetVatReport failed: %v", err)
	}

	// Kontrollera Boxar
	if report.Boxes.Box05 != 1000 { t.Errorf("Expected Box05=1000, got %d", report.Boxes.Box05) }
	if report.Boxes.Box06 != 500 { t.Errorf("Expected Box06=500, got %d", report.Boxes.Box06) }
	if report.Boxes.Box10 != 250 { t.Errorf("Expected Box10=250, got %d", report.Boxes.Box10) }
	if report.Boxes.Box11 != 60 { t.Errorf("Expected Box11=60, got %d", report.Boxes.Box11) }
	if report.TotalSales != 1500 { t.Errorf("Expected TotalSales=1500, got %d", report.TotalSales) }
	if report.OutgoingVat != 310 { t.Errorf("Expected OutgoingVat=310, got %d", report.OutgoingVat) }
	if report.NetVat != 310 { t.Errorf("Expected NetVat=310, got %d", report.NetVat) }
}

func TestGetVatReport_EUReverseCharge(t *testing.T) {
	l := setupVatTestDB(t)

	// Inköp från EU (t.ex. Vercel Hosting) 1000kr (25% moms). 
	// Kostnad bokat på 4531, betalt 1000kr från 1930.
	// Fiktiv moms: 2614 (ut) 250kr, 2645 (in) 250kr.

	_, err := l.PostVerification("System", models.VerificationRequest{
		Date: "2026-02-15",
		Text: "Vercel Hosting",
		Rows: []models.RowRequest{
			{Account: "1930", Kredit: 1000},
			{Account: "4531", Debet: 1000}, // Ruta 21
			{Account: "2645", Debet: 250},  // Ruta 48
			{Account: "2614", Kredit: 250}, // Ruta 30
		},
	})
	if err != nil {
		t.Fatalf("V1 failed: %v", err)
	}

	report, err := l.GetVatReport("2026-02-01", "2026-02-28")
	if err != nil {
		t.Fatalf("GetVatReport failed: %v", err)
	}

	if report.Boxes.Box21 != 1000 { t.Errorf("Expected Box21=1000, got %d", report.Boxes.Box21) }
	if report.Boxes.Box30 != 250 { t.Errorf("Expected Box30=250, got %d", report.Boxes.Box30) }
	if report.Boxes.Box48 != 250 { t.Errorf("Expected Box48=250, got %d", report.Boxes.Box48) }
	
	// Momseffekten av reverse charge är noll!
	if report.NetVat != 0 { t.Errorf("Expected NetVat=0, got %d", report.NetVat) }
}

func TestTransferVat_Zeroing(t *testing.T) {
	l := setupVatTestDB(t)

	// 1. Skapa ingående (500) och utgående (1000) moms
	_, err := l.PostVerification("System", models.VerificationRequest{
		Date: "2026-03-10",
		Text: "Försäljning 25%",
		Rows: []models.RowRequest{
			{Account: "1930", Debet: 5000},
			{Account: "2611", Kredit: 1000}, // Ut
			{Account: "3001", Kredit: 4000},
		},
	})
	if err != nil { t.Fatalf("V1 failed: %v", err) }

	_, err = l.PostVerification("System", models.VerificationRequest{
		Date: "2026-03-15",
		Text: "Inköp kontorsmateriel",
		Rows: []models.RowRequest{
			{Account: "1930", Kredit: 2500},
			{Account: "2641", Debet: 500}, // In
			{Account: "6110", Debet: 2000},
		},
	})
	if err != nil { t.Fatalf("V2 failed: %v", err) }

	// NetVat är 1000 (ut) - 500 (in) = 500 att betala (skuld på 2650)
	
	// Omför momsen
	err = l.TransferVat("System", "2026-03-01", "2026-03-31")
	if err != nil {
		t.Fatalf("TransferVat failed: %v", err)
	}

	// Kontrollera saldon direkt i databasen
	// 2611 ska vara 0, 2641 ska vara 0, 2650 ska vara -500 (kreditsaldo/skuld)
	balances := map[string]int64{}
	rows, _ := l.db.Query("SELECT account, SUM(debet)-SUM(kredit) FROM verification_rows WHERE account IN ('2611','2641','2650') GROUP BY account")
	defer rows.Close()
	for rows.Next() {
		var acc string
		var bal int64
		rows.Scan(&acc, &bal)
		balances[acc] = bal
	}

	if balances["2611"] != 0 { t.Errorf("Expected 2611 to be zero, got %d", balances["2611"]) }
	if balances["2641"] != 0 { t.Errorf("Expected 2641 to be zero, got %d", balances["2641"]) }
	if balances["2650"] != -500 { t.Errorf("Expected 2650 to be -500 (Skuld), got %d", balances["2650"]) }
}
