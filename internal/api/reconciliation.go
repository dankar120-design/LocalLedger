package api

import (
	"encoding/json"
	"net/http"

	"localledger/internal/models"
)

// handleGetReconciliationMatches anropar matchningsmotorn och returnerar förslagna kopplingar.
func (s *Server) handleGetReconciliationMatches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	matches, err := s.ledger.MatchBankTransactions()
	if err != nil {
		writeError(w, err)
		return
	}

	if matches == nil {
		matches = []models.ReconciliationMatch{} // Förhindra null i JSON
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(matches)
}
