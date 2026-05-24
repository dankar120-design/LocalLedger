package reports_test

import (
	"os"
	"strings"
	"testing"

	"localledger/internal/ledger"
	"localledger/internal/models"
	"localledger/internal/reports"
)

func TestGenerateSRUFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sru_test_*")
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

	// Bokför några transaktioner för att bygga upp ett saldo
	req := models.VerificationRequest{
		Date: "2026-01-15",
		Text: "Testförsäljning",
		Type: "NORMAL",
		Rows: []models.RowRequest{
			{Account: "1930", Debet: 125000, Kredit: 0},     // 1250.00 SEK tillgång
			{Account: "3001", Debet: 0, Kredit: 100000},    // 1000.00 SEK intäkt (moms 25%)
			{Account: "2611", Debet: 0, Kredit: 25000},     // 250.00 SEK utgående moms
		},
	}
	_, err = l.PostVerification("Test User", req)
	if err != nil {
		t.Fatalf("PostVerification failed: %v", err)
	}

	info, blanketter, err := reports.GenerateSRUFiles(l, 1)
	if err != nil {
		t.Fatalf("GenerateSRUFiles failed: %v", err)
	}

	infoStr := string(info)
	blanketterStr := string(blanketter)

	if !strings.Contains(infoStr, "#PRODUKT SRU") {
		t.Errorf("Expected info.sru to contain '#PRODUKT SRU', got:\n%s", infoStr)
	}
	if !strings.Contains(blanketterStr, "#BLANKETT NE_2026") {
		t.Errorf("Expected blanketter.sru to contain '#BLANKETT NE_2026', got:\n%s", blanketterStr)
	}

	// 3001 (Sales) mappar till SRU 7010 (Nettoomsättning). Förväntat saldo: 1000 SEK
	if !strings.Contains(blanketterStr, "#UPPGIFT 7010 1000") {
		t.Errorf("Expected blanketter.sru to contain '#UPPGIFT 7010 1000', got:\n%s", blanketterStr)
	}

	// 1930 (Checkkonto) mappar till SRU 7320 (Kassa och bank). Förväntat saldo: 1250 SEK
	if !strings.Contains(blanketterStr, "#UPPGIFT 7320 1250") {
		t.Errorf("Expected blanketter.sru to contain '#UPPGIFT 7320 1250', got:\n%s", blanketterStr)
	}
}
