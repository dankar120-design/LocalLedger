package ledger

import (
	"testing"

	"localledger/internal/models"
)

func TestSealVerifications(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		l, _ := OpenLedger(workspace, "v2.0.0")
		defer l.Close()

		// Försegla sandboxens befintliga rader så vi får en "ren" state för testet.
		_, _ = l.SealVerifications("Setup", false)

		// 1. Skapa tre verifikationer (ohashade)
		v1, _ := l.PostVerification("User", models.VerificationRequest{
			Date: "2023-03-01", Text: "Bokföring 1",
			Rows: []models.RowRequest{{Account: "1930", Debet: 1000, Kredit: 0}, {Account: "3010", Debet: 0, Kredit: 1000}},
		})
		_, _ = l.PostVerification("User", models.VerificationRequest{
			Date: "2023-03-02", Text: "Bokföring 2",
			Rows: []models.RowRequest{{Account: "1930", Debet: 2000, Kredit: 0}, {Account: "3010", Debet: 0, Kredit: 2000}},
		})
		v3, _ := l.PostVerification("User", models.VerificationRequest{
			Date: "2023-03-03", Text: "Bokföring 3",
			Rows: []models.RowRequest{{Account: "1930", Debet: 3000, Kredit: 0}, {Account: "3010", Debet: 0, Kredit: 3000}},
		})

		// 2. Försegla våra tre
		res, err := l.SealVerifications("User", false)
		if err != nil {
			t.Fatalf("SealVerifications failed: %v", err)
		}

		if res.Count != 3 {
			t.Errorf("Expected 3 sealed verifications, got %d", res.Count)
		}
		if len(res.LastHash) != 64 {
			t.Errorf("Expected 64 char hash, got %d chars: %s", len(res.LastHash), res.LastHash)
		}
		if res.FirstID != v1.ID || res.LastID != v3.ID {
			t.Errorf("Expected ID range %d-%d, got %d-%d", v1.ID, v3.ID, res.FirstID, res.LastID)
		}

		// 3. Verifiera kedjan
		valid, err := l.VerifyChain()
		if err != nil || !valid {
			t.Errorf("VerifyChain failed after sealing: %v", err)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		workspace := setupTestWorkspace(t)
		l, _ := OpenLedger(workspace, "v2.0.0")
		defer l.Close()

		// Försegla initial state
		_, _ = l.SealVerifications("Setup", false)

		// Ingen ny verifikation, anropa Seal
		res, err := l.SealVerifications("User", false)
		if err != nil {
			t.Fatalf("SealVerifications failed: %v", err)
		}
		if res.Count != 0 {
			t.Errorf("Expected 0 count, got %d", res.Count)
		}
	})
}

func TestVerifyChain_Tampered(t *testing.T) {
	workspace := setupTestWorkspace(t)
	l, _ := OpenLedger(workspace, "v2.0.0")
	defer l.Close()

	// 1. Skapa och lås en verifikation
	v1, _ := l.PostVerification("User", models.VerificationRequest{
		Date: "2023-03-01", Text: "Bokföring Original",
		Rows: []models.RowRequest{{Account: "1930", Debet: 1000, Kredit: 0}, {Account: "3010", Debet: 0, Kredit: 1000}},
	})
	_, err := l.SealVerifications("User", false)
	if err != nil {
		t.Fatalf("Failed to seal: %v", err)
	}

	// 2. Vi måste stänga av triggers temporärt i vår anslutning för att fuska (vi agerar illvillig aktör)
	// modernc.org/sqlite ignorerar triggers om man droppar dem.
	_, err = l.db.Exec("DROP TRIGGER protect_sealed_verifications")
	if err != nil {
		t.Fatalf("Failed to drop trigger for testing: %v", err)
	}

	// 3. Fuska! Ändra texten i databasen.
	_, err = l.db.Exec("UPDATE verifications SET text = 'Bokföring Manipulerad' WHERE id = ?", v1.ID)
	if err != nil {
		t.Fatalf("Failed to tamper DB: %v", err)
	}

	// 4. Verifiera (Revisorsknappen)
	valid, err := l.VerifyChain()
	if valid || err == nil {
		t.Errorf("Expected VerifyChain to fail due to tampering, but it succeeded!")
	} else {
		// Logga felet bara för att se hur det ser ut
		t.Logf("Tampering detected correctly: %v", err)
	}
}

func TestDatabaseTrigger_BlocksUpdate(t *testing.T) {
	workspace := setupTestWorkspace(t)
	l, _ := OpenLedger(workspace, "v2.0.0")
	defer l.Close()

	// 1. Skapa och lås
	v1, _ := l.PostVerification("User", models.VerificationRequest{
		Date: "2023-03-01", Text: "Riktig Bokföring",
		Rows: []models.RowRequest{{Account: "1930", Debet: 1000, Kredit: 0}, {Account: "3010", Debet: 0, Kredit: 1000}},
	})
	l.SealVerifications("User", false)

	// 2. Försök uppdatera (utan att droppa triggern)
	_, err := l.db.Exec("UPDATE verifications SET text = 'Fusk' WHERE id = ?", v1.ID)
	if err == nil {
		t.Errorf("Expected SQLite trigger to block the update, but it succeeded!")
	} else {
		t.Logf("Trigger blocked update successfully: %v", err)
	}
}
