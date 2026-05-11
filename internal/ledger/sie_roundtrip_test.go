package ledger

import (
	"localledger/internal/models"
	"testing"
)

func TestSIERoundtrip(t *testing.T) {
	// Skapa originaldatabasen
	dir1 := t.TempDir()
	if err := InitWorkspace(dir1); err != nil {
		t.Fatalf("Failed to init workspace: %v", err)
	}
	l1, err := OpenLedger(dir1, "v1.4.0")
	if err != nil {
		t.Fatalf("Failed to open original ledger: %v", err)
	}
	defer l1.Close()

	// Skapa år 2024 i l1
	_, err = l1.db.Exec("INSERT INTO fiscal_years (start_date, end_date) VALUES ('2024-01-01', '2024-12-31')")
	if err != nil {
		t.Fatalf("Failed to create fiscal year: %v", err)
	}

	// Hämta Year ID
	var yearID1 int64
	l1.db.QueryRow("SELECT id FROM fiscal_years WHERE start_date = '2024-01-01'").Scan(&yearID1)

	// Lägg in konton
	l1.db.Exec("INSERT INTO accounts (code, name, type) VALUES ('1910', 'Kassa', 'Tillgång'), ('3000', 'Försäljning', 'Intäkt'), ('4000', 'Inköp', 'Kostnad')")

	// Lägg in en komplex serie verifikationer i l1
	tx, _ := l1.db.Begin()
	reqs := []models.VerificationRequest{
		{
			Date: "2024-01-01",
			Text: "Ingående Balans",
			Type: "IB",
			Rows: []models.RowRequest{
				{Account: "1910", Debet: 50000},  // IB Kassa
				{Account: "2010", Kredit: 50000}, // IB Eget kapital
			},
		},
		{
			Date: "2024-01-15",
			Text: "Ociterad text simulering (mellanslag i namn)",
			Type: "NORMAL",
			Rows: []models.RowRequest{
				{Account: "1910", Debet: 10000}, // 100 kr in på Kassa
				{Account: "3000", Kredit: 10000}, // 100 kr försäljning
			},
		},
		{
			Date: "2024-01-20",
			Text: "Inköp med moms",
			Type: "NORMAL",
			Rows: []models.RowRequest{
				{Account: "4000", Debet: 8000},
				{Account: "1910", Kredit: 8000},
			},
		},
	}
	
	for _, req := range reqs {
		_, _, err = l1.postVerificationTx(tx, "TestUser", req)
		if err != nil {
			t.Fatalf("Failed to post ver: %v", err)
		}
	}
	tx.Commit()

	// Exportera SIE-4 från l1
	sieData, err := l1.GenerateSIE4(yearID1)
	if err != nil {
		t.Fatalf("Failed to generate SIE-4: %v", err)
	}

	// Skapa den nya databasen att importera till
	dir2 := t.TempDir()
	if err := InitWorkspace(dir2); err != nil {
		t.Fatalf("Failed to init destination workspace: %v", err)
	}
	l2, err := OpenLedger(dir2, "v1.4.0")
	if err != nil {
		t.Fatalf("Failed to open destination ledger: %v", err)
	}
	defer l2.Close()

	// Skapa samma år i l2
	_, err = l2.db.Exec("INSERT INTO fiscal_years (start_date, end_date) VALUES ('2024-01-01', '2024-12-31')")
	if err != nil {
		t.Fatalf("Failed to create fiscal year in l2: %v", err)
	}
	var yearID2 int64
	l2.db.QueryRow("SELECT id FROM fiscal_years WHERE start_date = '2024-01-01'").Scan(&yearID2)

	// Importera datan!
	err = l2.ImportSIE4("TestUser", yearID2, sieData)
	if err != nil {
		t.Fatalf("ImportSIE4 failed: %v", err)
	}

	// Jämför saldona
	getBalances := func(l *Ledger, yID int64) map[string]int64 {
		b := make(map[string]int64)
		rows, _ := l.db.Query("SELECT vr.account, SUM(vr.debet - vr.kredit) FROM verification_rows vr JOIN verifications v ON vr.verification_id = v.id GROUP BY vr.account")
		defer rows.Close()
		for rows.Next() {
			var acc string
			var bal int64
			rows.Scan(&acc, &bal)
			if bal != 0 {
				b[acc] = bal
			}
		}
		return b
	}

	bal1 := getBalances(l1, yearID1)
	bal2 := getBalances(l2, yearID2)

	if len(bal1) != len(bal2) {
		t.Fatalf("Balance mismatch! L1 has %d accounts with balances, L2 has %d", len(bal1), len(bal2))
	}

	for acc, val1 := range bal1 {
		if bal2[acc] != val1 {
			t.Errorf("Account %s mismatch: original %d, imported %d", acc, val1, bal2[acc])
		}
	}
}
