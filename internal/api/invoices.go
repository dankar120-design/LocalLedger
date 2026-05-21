package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"localledger/internal/models"
)

func (s *Server) HandleGetInvoices(w http.ResponseWriter, r *http.Request) {
	yearID, err := s.ledger.GetActiveFiscalYear()
	if err != nil || yearID == nil {
		http.Error(w, "Inget aktivt räkenskapsår", http.StatusBadRequest)
		return
	}

	invoices, err := s.ledger.GetInvoices(yearID.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Sätt standardvärde om det är nil
	if invoices == nil {
		invoices = []models.Invoice{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(invoices)
}

func (s *Server) HandleCreateInvoice(w http.ResponseWriter, r *http.Request) {
	var inv models.Invoice
	if err := json.NewDecoder(r.Body).Decode(&inv); err != nil {
		http.Error(w, "Ogiltig data", http.StatusBadRequest)
		return
	}

	yearID, err := s.ledger.GetActiveFiscalYear()
	if err != nil || yearID == nil {
		http.Error(w, "Inget aktivt räkenskapsår", http.StatusBadRequest)
		return
	}
	inv.FiscalYearID = yearID.ID

	// SECURITY: Ensure external API cannot spoof credit invoices
	inv.CreditOf = nil

	id, err := s.ledger.CreateInvoice(inv)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}

func (s *Server) HandleGetInvoice(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Ogiltigt ID", http.StatusBadRequest)
		return
	}

	inv, err := s.ledger.GetInvoiceByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(inv)
}

func (s *Server) HandleUpdateInvoice(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Ogiltigt ID", http.StatusBadRequest)
		return
	}

	var inv models.Invoice
	if err := json.NewDecoder(r.Body).Decode(&inv); err != nil {
		http.Error(w, "Ogiltig data", http.StatusBadRequest)
		return
	}
	inv.ID = id

	yearID, err := s.ledger.GetActiveFiscalYear()
	if err != nil || yearID == nil {
		http.Error(w, "Inget aktivt räkenskapsår", http.StatusBadRequest)
		return
	}
	inv.FiscalYearID = yearID.ID

	if err := s.ledger.UpdateInvoice(inv); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) HandleDeleteInvoice(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Ogiltigt ID", http.StatusBadRequest)
		return
	}

	if err := s.ledger.DeleteInvoice(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) HandlePostInvoice(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Ogiltigt ID", http.StatusBadRequest)
		return
	}

	// Assuming session-based user authentication later, currently hardcoded "System"
	user := "System" 

	if err := s.ledger.PostInvoice(id, user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

type PaymentRequest struct {
	Date string `json:"date"`
}

func (s *Server) HandlePayInvoice(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Ogiltigt ID", http.StatusBadRequest)
		return
	}

	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Ogiltig data", http.StatusBadRequest)
		return
	}

	user := "System" 

	if err := s.ledger.RegisterPayment(id, req.Date, user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) HandleGetInvoicePDF(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Ogiltigt ID", http.StatusBadRequest)
		return
	}

	inv, err := s.ledger.GetInvoiceByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if inv.VerificationID == nil {
		http.Error(w, "Fakturan är inte bokförd", http.StatusNotFound)
		return
	}

	hash, err := s.ledger.GetVerificationAttachmentHash(*inv.VerificationID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if hash == nil {
		http.Error(w, "Inget underlag hittades", http.StatusNotFound)
		return
	}

	http.Redirect(w, r, "/api/attachments/"+*hash, http.StatusTemporaryRedirect)
}

func (s *Server) HandleCreditInvoice(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Ogiltigt ID", http.StatusBadRequest)
		return
	}

	user := "System"
	newID, err := s.ledger.CreateCreditInvoice(id, user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]int64{"id": newID})
}

func (s *Server) HandleSettleInvoice(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Ogiltigt ID", http.StatusBadRequest)
		return
	}

	if err := s.ledger.SettleInvoice(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
