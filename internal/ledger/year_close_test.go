package ledger

import (
	"strings"
	"testing"
	"database/sql"

	"localledger/internal/models"
)

func setupTestDBForYearClose(t *testing.T) *Ledger {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open memory DB: %v", err)
	}

	// Kör initierings-schema (inkluderar nu BAS K1 och v11 konton)
	if err := runMigrations(db); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	l := &Ledger{db: db}

	// Create fiscal years
	_, err = l.db.Exec(`
		DELETE FROM fiscal_years;
		INSERT INTO fiscal_years (id, start_date, end_date) VALUES 
		(1, '2026-01-01', '2026-12-31'),
		(2, '2027-01-01', '2027-12-31');
	`)
	if err != nil {
		t.Fatalf("Failed to create fiscal years: %v", err)
	}

	return l
}

func addVerification(t *testing.T, l *Ledger, date string, rows []models.RowRequest) {
	tx, err := l.db.Begin()
	if err != nil {
		t.Fatalf("Failed to begin tx: %v", err)
	}

	req := models.VerificationRequest{
		Date: date,
		Text: "Test verification",
		Type: "NORMAL",
		Rows: rows,
	}

	_, _, err = l.postVerificationTx(tx, "Tester", req)
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to post verification: %v", err)
	}
	tx.Commit()
}

func TestGenerateOpeningBalance_BlockedWhenResultNotBooked(t *testing.T) {
	l := setupTestDBForYearClose(t)
	defer l.Close()

	// Lägg till intäkter, men gör inget bokslut.
	// 1930 Debet 10000, 3001 Kredit 10000
	addVerification(t, l, "2026-06-01", []models.RowRequest{
		{Account: "1930", Debet: 10000, Kredit: 0},
		{Account: "3001", Debet: 0, Kredit: 10000},
	})

	err := l.GenerateOpeningBalance("Tester", 1, 2)
	if err == nil {
		t.Fatalf("Expected error when generating IB without booked result, got nil")
	}

	if !strings.Contains(err.Error(), "balansräkningen balanserar inte") {
		t.Errorf("Expected balance error, got: %v", err)
	}
}

func TestGenerateOpeningBalance_SuccessWhenResultBooked(t *testing.T) {
	l := setupTestDBForYearClose(t)
	defer l.Close()

	// 1. Lägg till intäkter
	addVerification(t, l, "2026-06-01", []models.RowRequest{
		{Account: "1930", Debet: 10000, Kredit: 0},
		{Account: "3001", Debet: 0, Kredit: 10000},
	})

	// 2. Gör manuellt bokslut för året
	addVerification(t, l, "2026-12-31", []models.RowRequest{
		{Account: "8999", Debet: 10000, Kredit: 0},
		{Account: "2099", Debet: 0, Kredit: 10000},
	})

	err := l.GenerateOpeningBalance("Tester", 1, 2)
	if err != nil {
		t.Fatalf("Expected success when result is booked, got: %v", err)
	}

	// Verifiera att IB har skapats i år 2 (2027)
	var ibExists bool
	err = l.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM verifications WHERE date >= '2027-01-01' AND date <= '2027-12-31' AND type = 'IB'
		)
	`).Scan(&ibExists)
	if err != nil {
		t.Fatalf("Failed to query for IB: %v", err)
	}
	if !ibExists {
		t.Errorf("IB verification was not created in the target year")
	}

	// Kontrollera saldon i IB-verifikationen
	rows, err := l.db.Query(`
		SELECT r.account, r.debet, r.kredit
		FROM verification_rows r
		JOIN verifications v ON r.verification_id = v.id
		WHERE v.type = 'IB' AND v.date = '2027-01-01'
	`)
	if err != nil {
		t.Fatalf("Failed to query IB rows: %v", err)
	}
	defer rows.Close()

	found1930 := false
	found2099 := false

	for rows.Next() {
		var acc string
		var d, k int64
		rows.Scan(&acc, &d, &k)
		if acc == "1930" && d == 10000 && k == 0 {
			found1930 = true
		}
		if acc == "2099" && d == 0 && k == 10000 {
			found2099 = true
		}
	}

	if !found1930 || !found2099 {
		t.Errorf("IB verification did not contain the correct balances. 1930: %v, 2099: %v", found1930, found2099)
	}
}

func TestGenerateOpeningBalance_LedgerCorruption(t *testing.T) {
	l := setupTestDBForYearClose(t)
	defer l.Close()

	// Injicera korrupt data direkt via SQL (förbigå postVerificationTx säkerhetsspärrar)
	_, err := l.db.Exec(`
		INSERT INTO verifications (id, date, text, type) VALUES (999, '2026-06-01', 'Corrupt Data', 'NORMAL');
		INSERT INTO verification_rows (verification_id, account, debet, kredit) VALUES 
		(999, '1930', 5000, 0); -- Ensidig bokföring! Endast Debet!
	`)
	if err != nil {
		t.Fatalf("Failed to inject corrupt data: %v", err)
	}

	err = l.GenerateOpeningBalance("Tester", 1, 2)
	if err == nil {
		t.Fatalf("Expected LEDGER CORRUPTION error, got nil")
	}

	if !strings.Contains(err.Error(), "LEDGER CORRUPTION") {
		t.Errorf("Expected LEDGER CORRUPTION error, got: %v", err)
	}
}
