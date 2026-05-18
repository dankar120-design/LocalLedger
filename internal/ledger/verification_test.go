package ledger

import (
	"errors"
	"testing"

	"localledger/internal/models"
)

func TestPostVerification(t *testing.T) {

	validReq := models.VerificationRequest{
		Date: "2023-02-01",
		Text: "Test verifikation",
		Rows: []models.RowRequest{
			{Account: "1930", Debet: 100000, Kredit: 0},
			{Account: "3010", Debet: 0, Kredit: 100000},
		},
	}

	t.Run("Success", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		l, _ := OpenLedger(workspace, "v2.0.0")
		defer l.Close()

		res, err := l.PostVerification("SystemTest", validReq)
		if err != nil {
			t.Errorf("Expected success, got error: %v", err)
		}
		if res == nil || res.ID == 0 {
			t.Error("Expected valid VerificationResult")
		}
	})

	t.Run("Invalid Date Format", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		l, _ := OpenLedger(workspace, "v2.0.0")
		defer l.Close()

		req := validReq
		req.Date = "2023-2-1" // Ogiltigt format
		_, err := l.PostVerification("SystemTest", req)
		if !errors.Is(err, ErrValidation) {
			t.Errorf("Expected ErrValidation, got: %v", err)
		}
	})

	t.Run("Invalid Rows (Length < 2)", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		l, _ := OpenLedger(workspace, "v2.0.0")
		defer l.Close()

		req := validReq
		req.Rows = []models.RowRequest{{Account: "1930", Debet: 0, Kredit: 0}}
		_, err := l.PostVerification("SystemTest", req)
		if !errors.Is(err, ErrValidation) {
			t.Errorf("Expected ErrValidation, got: %v", err)
		}
	})

	t.Run("Negative amounts", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		l, _ := OpenLedger(workspace, "v2.0.0")
		defer l.Close()

		req := validReq
		req.Rows = []models.RowRequest{
			{Account: "1930", Debet: -100, Kredit: 0},
			{Account: "3010", Debet: 0, Kredit: -100},
		}
		_, err := l.PostVerification("SystemTest", req)
		if !errors.Is(err, ErrValidation) {
			t.Errorf("Expected ErrValidation, got: %v", err)
		}
	})

	t.Run("Zero amounts", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		l, _ := OpenLedger(workspace, "v2.0.0")
		defer l.Close()

		req := validReq
		req.Rows = []models.RowRequest{
			{Account: "1930", Debet: 0, Kredit: 0},
			{Account: "3010", Debet: 0, Kredit: 0},
		}
		_, err := l.PostVerification("SystemTest", req)
		if !errors.Is(err, ErrValidation) {
			t.Errorf("Expected ErrValidation, got: %v", err)
		}
	})

	t.Run("Unbalanced", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		l, _ := OpenLedger(workspace, "v2.0.0")
		defer l.Close()

		req := validReq
		req.Rows = []models.RowRequest{
			{Account: "1930", Debet: 100000, Kredit: 0},
			{Account: "3010", Debet: 0, Kredit: 90000},
		}
		_, err := l.PostVerification("SystemTest", req)
		if !errors.Is(err, ErrValidation) {
			t.Errorf("Expected ErrValidation, got: %v", err)
		}
	})

	t.Run("No Fiscal Year", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		l, _ := OpenLedger(workspace, "v2.0.0")
		defer l.Close()

		req := validReq
		req.Date = "2025-01-01" // Finns inget räkenskapsår för detta datum i sandboxen
		_, err := l.PostVerification("SystemTest", req)
		if !errors.Is(err, ErrNoFiscalYear) {
			t.Errorf("Expected ErrNoFiscalYear, got: %v", err)
		}
	})

	t.Run("Locked Period", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		l, _ := OpenLedger(workspace, "v2.0.0")
		defer l.Close()

		// Vi skapar ett lås för 2023-03 manuellt för att testa
		_, err := l.db.Exec("INSERT INTO period_locks (year_month, locked_by, locked_at) VALUES ('2023-03', 'SystemTest', datetime('now'))")
		if err != nil {
			t.Fatalf("Failed to setup period lock: %v", err)
		}

		req := validReq
		req.Date = "2023-03-15"
		_, err = l.PostVerification("SystemTest", req)
		if !errors.Is(err, ErrPeriodLocked) {
			t.Errorf("Expected ErrPeriodLocked, got: %v", err)
		}
	})

	t.Run("Invalid Account", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		l, _ := OpenLedger(workspace, "v2.0.0")
		defer l.Close()

		req := validReq
		req.Rows = []models.RowRequest{
			{Account: "9999", Debet: 100000, Kredit: 0}, // Konto 9999 finns inte i sandbox
			{Account: "3010", Debet: 0, Kredit: 100000},
		}
		_, err := l.PostVerification("SystemTest", req)
		if err == nil {
			t.Error("Expected error for foreign key constraint violation, got nil")
		}
	})
}
