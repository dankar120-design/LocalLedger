package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"localledger/internal/models"
)

func performRequest(srv *Server, method, path string, body interface{}) *httptest.ResponseRecorder {
	var reqBody []byte
	if body != nil {
		reqBody, _ = json.Marshal(body)
	}

	req := httptest.NewRequest(method, path, bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer "+srv.Token())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	rr := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rr, req)
	return rr
}

func TestPostVerificationAPI(t *testing.T) {
	workspace := setupTestWorkspace(t)
	srv, _ := Start(workspace, 0)
	defer srv.ledger.Close()
	defer srv.httpServer.Close()

	// 1. Success
	reqBody := models.VerificationRequest{
		Date: "2023-01-15", Text: "Kaffe",
		Rows: []models.RowRequest{
			{Account: "6110", Debet: 100, Kredit: 0},
			{Account: "1930", Debet: 0, Kredit: 100},
		},
	}
	rr := performRequest(srv, http.MethodPost, "/api/verifications", reqBody)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected 201 Created, got %v: %s", rr.Code, rr.Body.String())
	}

	// 2. Obalanserad -> 400 Bad Request
	reqBody.Rows[0].Debet = 200
	rr = performRequest(srv, http.MethodPost, "/api/verifications", reqBody)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for unbalanced, got %v", rr.Code)
	}

	// 3. Stängd period -> 403 Forbidden
	// Lås perioden först
	performRequest(srv, http.MethodPost, "/api/lock/period", map[string]string{"year_month": "2023-01"})
	
	// Försök posta igen (nu balanserad)
	reqBody.Rows[0].Debet = 100
	rr = performRequest(srv, http.MethodPost, "/api/verifications", reqBody)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for locked period, got %v", rr.Code)
	}
}

func TestLockPeriodAPI(t *testing.T) {
	workspace := setupTestWorkspace(t)
	srv, _ := Start(workspace, 0)
	defer srv.ledger.Close()
	defer srv.httpServer.Close()

	// Lås period 2023-01
	rr := performRequest(srv, http.MethodPost, "/api/lock/period", map[string]string{"year_month": "2023-01"})
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %v", rr.Code)
	}

	// Lås samma igen -> 409 Conflict
	rr = performRequest(srv, http.MethodPost, "/api/lock/period", map[string]string{"year_month": "2023-01"})
	if rr.Code != http.StatusConflict {
		t.Errorf("Expected 409 Conflict for double lock, got %v", rr.Code)
	}
}

func TestGetVerificationsAPI(t *testing.T) {
	workspace := setupTestWorkspace(t)
	srv, _ := Start(workspace, 0)
	defer srv.ledger.Close()
	defer srv.httpServer.Close()

	rr := performRequest(srv, http.MethodGet, "/api/verifications", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %v", rr.Code)
	}

	var verifications []models.VerificationResponse
	if err := json.NewDecoder(rr.Body).Decode(&verifications); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Sandboxen har 10 verifikationer
	if len(verifications) != 10 {
		t.Errorf("Expected 10 verifications, got %d", len(verifications))
	}
	// Första verifikationen (id 1) borde ha 2 rader
	if len(verifications) > 0 && len(verifications[0].Rows) != 2 {
		t.Errorf("Expected 2 rows on first verification, got %d", len(verifications[0].Rows))
	}
}
