package ledger

import (
	"errors"
	"testing"

	"localledger/internal/models"
)

func TestLockPeriod(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		l, _ := OpenLedger(workspace, "v2.0.0")
		defer l.Close()

		// Försegla sandboxens befintliga rader så vi får en "ren" state för testet.
		_, _ = l.SealVerifications("Setup", false)

		// 1. Skapa en verifikation
		v1, _ := l.PostVerification("User", models.VerificationRequest{
			Date: "2023-01-15", Text: "Bokföring Januari",
			Rows: []models.RowRequest{{Account: "1930", Debet: 1000, Kredit: 0}, {Account: "3010", Debet: 0, Kredit: 1000}},
		})

		// 2. Lås perioden
		res, err := l.LockPeriod("2023-01", "Admin")
		if err != nil {
			t.Fatalf("LockPeriod failed: %v", err)
		}

		if res.Count != 1 {
			t.Errorf("Expected 1 verification to be sealed, got %d", res.Count)
		}
		if res.FirstID != v1.ID {
			t.Errorf("Expected sealed verification to be ID %d, got %d", v1.ID, res.FirstID)
		}

		// 3. Verifiera att det inte går att posta till den låsta perioden nu
		_, err = l.PostVerification("User", models.VerificationRequest{
			Date: "2023-01-20", Text: "Försök att posta i låst",
			Rows: []models.RowRequest{{Account: "1930", Debet: 1000, Kredit: 0}, {Account: "3010", Debet: 0, Kredit: 1000}},
		})
		if !errors.Is(err, ErrPeriodLocked) {
			t.Errorf("Expected ErrPeriodLocked, got %v", err)
		}
	})

	t.Run("InvalidFormat", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		l, _ := OpenLedger(workspace, "v2.0.0")
		defer l.Close()

		_, err := l.LockPeriod("2023-13", "Admin")
		if err != ErrInvalidPeriodFormat {
			t.Errorf("Expected ErrInvalidPeriodFormat for 2023-13, got: %v", err)
		}

		_, err = l.LockPeriod("23-01", "Admin")
		if err != ErrInvalidPeriodFormat {
			t.Errorf("Expected ErrInvalidPeriodFormat for 23-01, got: %v", err)
		}
	})

	t.Run("AlreadyLocked", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		l, _ := OpenLedger(workspace, "v2.0.0")
		defer l.Close()

		_, err := l.LockPeriod("2023-02", "Admin")
		if err != nil {
			t.Fatalf("First LockPeriod failed: %v", err)
		}

		_, err = l.LockPeriod("2023-02", "Admin")
		if err != ErrPeriodAlreadyLocked {
			t.Errorf("Expected ErrPeriodAlreadyLocked, got: %v", err)
		}
	})
}

func TestLockFiscalYear(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		l, _ := OpenLedger(workspace, "v2.0.0")
		defer l.Close()

		// Sandboxen skapade Fiscal Year med ID 1 (2023-01-01 -> 2023-12-31)
		
		// Lås året
		res, err := l.LockFiscalYear(1, "Admin")
		if err != nil {
			t.Fatalf("LockFiscalYear failed: %v", err)
		}

		// I sandboxen finns det 10 initiala verifikationer
		if res.Count != 10 {
			t.Errorf("Expected 10 verifications to be sealed upon locking year, got %d", res.Count)
		}

		// Lås igen -> Error
		_, err = l.LockFiscalYear(1, "Admin")
		if err != ErrFiscalYearLocked {
			t.Errorf("Expected ErrFiscalYearLocked, got: %v", err)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		l, _ := OpenLedger(workspace, "v2.0.0")
		defer l.Close()

		_, err := l.LockFiscalYear(999, "Admin")
		if err != ErrFiscalYearNotFound {
			t.Errorf("Expected ErrFiscalYearNotFound, got: %v", err)
		}
	})
}
