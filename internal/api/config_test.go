package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakePathRelativeIfPossible(t *testing.T) {
	// Spara det ursprungliga exe-sökvägstestet
	// Skapa en temporär exekverbar arbetskatalog
	tempDir, err := os.MkdirTemp("", "localledger_config_test_*")
	if err != nil {
		t.Fatalf("Kunde inte skapa tempdir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Eftersom vi inte enkelt kan ändra os.Executable() under körning utan att påverka resten,
	// testar vi logiken genom att verifiera makePathRelativeIfPossible och makePathAbsolute
	// mot den faktiska exe-katalogen.
	exeDir := getExeDir()
	if exeDir == "" {
		t.Skip("Kunde inte hämta exe-katalog, hoppar över")
	}

	absExeDir, err := filepath.Abs(exeDir)
	if err != nil {
		t.Fatalf("Kunde inte hämta absolut exeDir: %v", err)
	}

	// 1. Test av mapp på samma enhet (bör bli relativ)
	targetOnSameDrive := filepath.Join(absExeDir, "test_workspace")
	relPath := makePathRelativeIfPossible(targetOnSameDrive)
	if filepath.IsAbs(relPath) && filepath.VolumeName(absExeDir) == filepath.VolumeName(targetOnSameDrive) {
		t.Errorf("Förväntade relativ sökväg för %s, fick absolut %s", targetOnSameDrive, relPath)
	}

	// 2. Återskapa till absolut
	resolvedAbs := makePathAbsolute(relPath)
	absExpected, _ := filepath.Abs(targetOnSameDrive)
	if !strings.EqualFold(resolvedAbs, absExpected) {
		t.Errorf("makePathAbsolute felaktig. Förväntade %s, fick %s", absExpected, resolvedAbs)
	}
}

func TestGetConfigPath(t *testing.T) {
	path, isPortable := getConfigPath()
	if path == "" {
		t.Fatal("getConfigPath returnerade tom sökväg")
	}

	// Verifiera att om isPortable är true, så ligger filen bredvid binären
	if isPortable {
		exeDir := getExeDir()
		expectedDir, err := filepath.Abs(exeDir)
		if err == nil {
			actualDir := filepath.Dir(path)
			if !strings.EqualFold(actualDir, expectedDir) {
				t.Errorf("Bärbart läge aktivt men konfigurationsfilen sparades inte i exe-katalogen. Förväntade %s, fick %s", expectedDir, actualDir)
			}
		}
	}
}

func TestSaveAndLoadGlobalConfig(t *testing.T) {
	// Skapa temporär arbetskatalog
	tempDir, err := os.MkdirTemp("", "localledger_saveload_test_*")
	if err != nil {
		t.Fatalf("Kunde inte skapa tempdir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Använd en anpassad temporär miljö för att inte skriva över användarens riktiga config
	// Vi byter ut LOCALAPPDATA tillfälligt under testet
	origLocalAppData := os.Getenv("LOCALAPPDATA")
	os.Setenv("LOCALAPPDATA", tempDir)
	defer os.Setenv("LOCALAPPDATA", origLocalAppData)

	// Eftersom getConfigPath() kollar om exeDir är skrivbar (vilket den oftast är under go test i lokala mappar),
	// sparar vi direkt i en config-fil under testet och laddar den med LoadGlobalConfig.
	
	testWorkspace := filepath.Join(tempDir, "Foretag_AB")
	err = os.MkdirAll(testWorkspace, 0755)
	if err != nil {
		t.Fatalf("Kunde inte skapa testmapp: %v", err)
	}

	// Skapa en ledger.db så den inte markeras som Orphaned
	dbFile, err := os.Create(filepath.Join(testWorkspace, "ledger.db"))
	if err != nil {
		t.Fatalf("Kunde inte skapa dummy db: %v", err)
	}
	dbFile.Close()

	// Spara konfig
	err = SaveGlobalConfig(testWorkspace, "Testföretaget AB")
	if err != nil {
		t.Fatalf("SaveGlobalConfig misslyckades: %v", err)
	}

	// Ladda konfig och verifiera
	config, _, _ := LoadGlobalConfig()
	if len(config.RecentWorkspaces) == 0 {
		t.Fatal("RecentWorkspaces var tom efter sparning")
	}

	found := false
	for _, w := range config.RecentWorkspaces {
		if strings.EqualFold(w.Path, testWorkspace) {
			found = true
			if w.CompanyName != "Testföretaget AB" {
				t.Errorf("Felaktigt företagsnamn. Förväntade 'Testföretaget AB', fick '%s'", w.CompanyName)
			}
			if w.Orphaned {
				t.Error("Arbetsytan markerades felaktigt som Orphaned")
			}
		}
	}

	if !found {
		t.Errorf("Sparad arbetsyta %s hittades inte i listan", testWorkspace)
	}

	// Testa borttagning
	err = RemoveGlobalConfig(testWorkspace)
	if err != nil {
		t.Fatalf("RemoveGlobalConfig misslyckades: %v", err)
	}

	configAfterRemove, _, _ := LoadGlobalConfig()
	for _, w := range configAfterRemove.RecentWorkspaces {
		if strings.EqualFold(w.Path, testWorkspace) {
			t.Error("Arbetsytan fanns kvar efter att RemoveGlobalConfig körts")
		}
	}
}

func TestOrphanDetection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "localledger_orphan_test_*")
	if err != nil {
		t.Fatalf("Kunde inte skapa tempdir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	origLocalAppData := os.Getenv("LOCALAPPDATA")
	os.Setenv("LOCALAPPDATA", tempDir)
	defer os.Setenv("LOCALAPPDATA", origLocalAppData)

	// Skapa en arbetsyta som INTE har en ledger.db (ska markeras som Orphaned)
	orphanWorkspace := filepath.Join(tempDir, "Orphaned_Foretag")
	err = os.MkdirAll(orphanWorkspace, 0755)
	if err != nil {
		t.Fatalf("Kunde inte skapa testmapp: %v", err)
	}

	err = SaveGlobalConfig(orphanWorkspace, "Förlorat Företag AB")
	if err != nil {
		t.Fatalf("SaveGlobalConfig misslyckades: %v", err)
	}

	config, _, _ := LoadGlobalConfig()
	found := false
	for _, w := range config.RecentWorkspaces {
		if strings.EqualFold(w.Path, orphanWorkspace) {
			found = true
			if !w.Orphaned {
				t.Error("Förväntade att arbetsyta utan ledger.db skulle markeras som Orphaned=true")
			}
		}
	}

	if !found {
		t.Errorf("Sparad arbetsyta %s hittades inte", orphanWorkspace)
	}
}

func TestIsForbiddenDirectory(t *testing.T) {
	// Spara och manipulera miljövariabler för att testa deterministiskt
	origWindir := os.Getenv("WINDIR")
	defer os.Setenv("WINDIR", origWindir)

	os.Setenv("WINDIR", `C:\Windows`)

	tests := []struct {
		path     string
		expected bool
	}{
		{"", true},
		{`C:\Windows`, true},
		{`C:\Windows\System32`, true},
		{`C:\`, true},
		{`D:\`, true},
		{`C:\MinaDokument`, false},
		{`C:\Users\dka12\Documents\LocalLedger_Data`, false},
	}

	for _, tc := range tests {
		res := isForbiddenDirectory(tc.path)
		if res != tc.expected {
			t.Errorf("isForbiddenDirectory(%q) = %t; förväntade %t", tc.path, res, tc.expected)
		}
	}
}
