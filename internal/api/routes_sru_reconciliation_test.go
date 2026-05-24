package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"localledger/internal/models"
)

func TestSRUAndReconciliationRoutes(t *testing.T) {
	workspace := setupTestWorkspace(t)
	srv, err := Start(workspace, 0, false, true)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.ledger.Close()
	defer srv.httpServer.Close()

	// 1. Test handleGetReconciliationMatches route
	rrRec := performRequest(srv, http.MethodGet, "/api/reconciliation/match", nil)
	if rrRec.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for reconciliation match route, got %v: %s", rrRec.Code, rrRec.Body.String())
	}

	var matches []models.ReconciliationMatch
	if err := json.NewDecoder(rrRec.Body).Decode(&matches); err != nil {
		t.Errorf("Failed to decode matches JSON: %v", err)
	}

	// 2. Test handleExportSRU route
	rrSRU := performRequest(srv, http.MethodGet, "/api/export/sru", nil)
	if rrSRU.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for SRU export, got %v: %s", rrSRU.Code, rrSRU.Body.String())
	}

	contentType := rrSRU.Header().Get("Content-Type")
	if contentType != "application/zip" {
		t.Errorf("Expected Content-Type application/zip, got %q", contentType)
	}
}
