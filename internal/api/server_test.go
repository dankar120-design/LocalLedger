package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// setupTestWorkspace skapar en temporär mapp för databasen
func setupTestWorkspace(t *testing.T) string {
	tempDir := t.TempDir()
	
	baseSrcPath := filepath.Join("..", "..", "examples", "DemoForetaget_AB", "ledger.db")
	baseDstPath := filepath.Join(tempDir, "ledger.db")

	copyFile := func(src, dst string) {
		s, err := os.Open(src)
		if err != nil {
			if os.IsNotExist(err) {
				return // WAL/SHM might not exist
			}
			t.Fatalf("failed to open source db file %s: %v", src, err)
		}
		defer s.Close()

		d, err := os.Create(dst)
		if err != nil {
			t.Fatalf("failed to create temp db file %s: %v", dst, err)
		}
		defer d.Close()

		if _, err = io.Copy(d, s); err != nil {
			t.Fatalf("failed to copy file %s to %s: %v", src, dst, err)
		}
	}

	copyFile(baseSrcPath, baseDstPath)
	copyFile(baseSrcPath+"-wal", baseDstPath+"-wal")
	copyFile(baseSrcPath+"-shm", baseDstPath+"-shm")

	return tempDir
}

func TestHealthEndpoint(t *testing.T) {
	workspace := setupTestWorkspace(t)
	
	// Startar servern i bakgrunden. Den skapar en ny databas i workspace.
	srv, err := Start(workspace, 0, false, true)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	// Vi kan stänga servern manuellt utan context eftersom det är ett test.
	defer srv.ledger.Close()
	defer srv.httpServer.Close()

	// 1. Testa utan token (ska misslyckas)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	
	// Anropa serverns handler direkt (vi bypassar det riktiga nätverket)
	srv.httpServer.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %v", rr.Code)
	}

	// 2. Testa med fel token
	req = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Authorization", "Bearer WRONG_TOKEN")
	rr = httptest.NewRecorder()
	
	srv.httpServer.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %v", rr.Code)
	}

	// 3. Testa med RÄTT token
	req = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Authorization", "Bearer "+srv.Token())
	rr = httptest.NewRecorder()
	
	srv.httpServer.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %v. Body: %s", rr.Code, rr.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%v'", response["status"])
	}
}

func TestWorkspaceHash(t *testing.T) {
	workspace := setupTestWorkspace(t)

	// 1. Sandbox mode (isSandbox = true)
	{
		srv, err := Start(workspace, 0, false, true)
		if err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}
		
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		srv.httpServer.Handler.ServeHTTP(rr, req)
		srv.ledger.Close()
		srv.httpServer.Close()

		body := rr.Body.String()
		re := regexp.MustCompile(`<meta name="workspace-hash" content="([a-f0-9]+)">`)
		matches := re.FindStringSubmatch(body)
		if len(matches) < 2 {
			t.Fatalf("Sandbox: Expected to find workspace-hash meta tag in body: %s", body)
		}
		sandboxHash := matches[1]
		if len(sandboxHash) != 16 {
			t.Errorf("Sandbox: Expected 16-char hash, got: %s", sandboxHash)
		}
	}

	// 2. Production mode (isSandbox = false) with org_number and name
	{
		srv, err := Start(workspace, 0, false, false)
		if err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}

		// Let's modify company settings to have an OrgNumber
		settings, err := srv.ledger.GetSettings()
		if err != nil {
			t.Fatalf("Failed to get settings: %v", err)
		}
		settings.OrgNumber = "123456-7890"
		settings.Name = "Test Company"
		if err := srv.ledger.UpdateSettings(settings); err != nil {
			t.Fatalf("Failed to update settings: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		srv.httpServer.Handler.ServeHTTP(rr, req)
		srv.ledger.Close()
		srv.httpServer.Close()

		body := rr.Body.String()
		re := regexp.MustCompile(`<meta name="workspace-hash" content="([a-f0-9]+)">`)
		matches := re.FindStringSubmatch(body)
		if len(matches) < 2 {
			t.Fatalf("Production: Expected to find workspace-hash meta tag in body")
		}
		hash1 := matches[1]

		// Now change Name but keep OrgNumber the same - should produce the same hash!
		srv2, err := Start(workspace, 0, false, false)
		if err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}
		settings.Name = "Updated Company Name"
		if err := srv2.ledger.UpdateSettings(settings); err != nil {
			t.Fatalf("Failed to update settings: %v", err)
		}

		req2 := httptest.NewRequest(http.MethodGet, "/", nil)
		rr2 := httptest.NewRecorder()
		srv2.httpServer.Handler.ServeHTTP(rr2, req2)
		srv2.ledger.Close()
		srv2.httpServer.Close()

		body2 := rr2.Body.String()
		matches2 := re.FindStringSubmatch(body2)
		if len(matches2) < 2 {
			t.Fatalf("Production 2: Expected to find workspace-hash meta tag in body")
		}
		hash2 := matches2[1]

		if hash1 != hash2 {
			t.Errorf("Expected hashes to be identical after name change when OrgNumber is stable. Got %s and %s", hash1, hash2)
		}
	}

	// 3. Fallback to name only (when OrgNumber is empty)
	{
		srv, err := Start(workspace, 0, false, false)
		if err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}

		settings, err := srv.ledger.GetSettings()
		if err != nil {
			t.Fatalf("Failed to get settings: %v", err)
		}
		settings.OrgNumber = ""
		settings.Name = "OnlyNameCompany"
		if err := srv.ledger.UpdateSettings(settings); err != nil {
			t.Fatalf("Failed to update settings: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		srv.httpServer.Handler.ServeHTTP(rr, req)
		srv.ledger.Close()
		srv.httpServer.Close()

		body := rr.Body.String()
		re := regexp.MustCompile(`<meta name="workspace-hash" content="([a-f0-9]+)">`)
		matches := re.FindStringSubmatch(body)
		if len(matches) < 2 {
			t.Fatalf("NameOnly: Expected to find workspace-hash meta tag in body")
		}
		nameOnlyHash := matches[1]
		if len(nameOnlyHash) != 16 {
			t.Errorf("Expected 16-char hash, got: %s", nameOnlyHash)
		}
	}

	// 4. Fallback to workspace path (when both are empty)
	{
		srv, err := Start(workspace, 0, false, false)
		if err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}

		settings, err := srv.ledger.GetSettings()
		if err != nil {
			t.Fatalf("Failed to get settings: %v", err)
		}
		settings.OrgNumber = ""
		settings.Name = ""
		if err := srv.ledger.UpdateSettings(settings); err != nil {
			t.Fatalf("Failed to update settings: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		srv.httpServer.Handler.ServeHTTP(rr, req)
		srv.ledger.Close()
		srv.httpServer.Close()

		body := rr.Body.String()
		re := regexp.MustCompile(`<meta name="workspace-hash" content="([a-f0-9]+)">`)
		matches := re.FindStringSubmatch(body)
		if len(matches) < 2 {
			t.Fatalf("Fallback: Expected to find workspace-hash meta tag in body")
		}
		fallbackHash := matches[1]
		if len(fallbackHash) != 16 {
			t.Errorf("Expected 16-char hash, got: %s", fallbackHash)
		}
	}
}
