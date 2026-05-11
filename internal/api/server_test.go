package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	srv, err := Start(workspace, 0)
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
