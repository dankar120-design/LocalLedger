package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"html/template"

	"localledger/internal/ledger"
	"localledger/internal/models"
)

// registerRoutes sätter upp alla HTTP handlers för API:et
func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/verifications", s.handleGetVerifications)
	mux.HandleFunc("POST /api/verifications", s.handlePostVerification)
	mux.HandleFunc("POST /api/verifications/{id}/storno", s.handleStornoVerification)
	mux.HandleFunc("POST /api/verifications/{id}/void", s.handleVoidDraftVerification)
	mux.HandleFunc("POST /api/lock/period", s.handleLockPeriod)
	mux.HandleFunc("GET /api/verify", s.handleVerifyChain)
	mux.HandleFunc("GET /api/fiscal-years", s.handleGetFiscalYears)
	mux.HandleFunc("POST /api/fiscal-years", s.handleCreateFiscalYear)
	mux.HandleFunc("POST /api/fiscal-years/{id}/lock", s.handleLockFiscalYear)
	mux.HandleFunc("POST /api/maintenance/fill-gaps", s.handleFillGaps)
	mux.HandleFunc("POST /api/maintenance/seal", s.handleSealVerifications)
	mux.HandleFunc("GET /api/attachments/{hash}", s.handleGetAttachment)
	mux.HandleFunc("GET /api/reports/samlingsplan", s.handleGetSamlingsplan)
	mux.HandleFunc("GET /api/accounts", s.handleGetAccounts)
	mux.HandleFunc("GET /reports", s.handleGetReports)
	mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	mux.HandleFunc("POST /api/settings", s.handlePostSettings)
	mux.HandleFunc("POST /api/accounts", s.handleAddAccount)
	mux.HandleFunc("GET /api/export/sie4", s.handleExportSIE4)
	mux.HandleFunc("GET /api/export/backup", s.handleExportBackup)
	mux.HandleFunc("GET /api/vat-report", s.handleGetVatReport)
	mux.HandleFunc("POST /api/vat-report/transfer", s.handleTransferVat)
}

// ErrorResponse är standardformatet för felmeddelanden
type ErrorResponse struct {
	Error string `json:"error"`
}

// writeError hanterar mappningen från Go-errors till HTTP Statuskoder
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError

	if errors.Is(err, ledger.ErrValidation) || errors.Is(err, ledger.ErrInvalidPeriodFormat) {
		status = http.StatusBadRequest
	} else if errors.Is(err, ledger.ErrPeriodLocked) || errors.Is(err, ledger.ErrFiscalYearLocked) {
		status = http.StatusForbidden
	} else if errors.Is(err, ledger.ErrPeriodAlreadyLocked) || errors.Is(err, ledger.ErrWormViolation) {
		status = http.StatusConflict
	} else if errors.Is(err, ledger.ErrNoFiscalYear) {
		status = http.StatusNotFound
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
}

func (s *Server) handleGetVerifications(w http.ResponseWriter, r *http.Request) {
	yearIDStr := r.URL.Query().Get("year_id")
	var yearID *int64
	if yearIDStr != "" {
		if id, err := strconv.ParseInt(yearIDStr, 10, 64); err == nil {
			yearID = &id
		}
	}

	verifications, err := s.ledger.GetVerifications(yearID)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(verifications)
}

func (s *Server) handlePostVerification(w http.ResponseWriter, r *http.Request) {
	// Begränsa payload till ca 15MB (10MB fil + Base64 overhead + JSON)
	r.Body = http.MaxBytesReader(w, r.Body, 15*1024*1024)

	var req models.VerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: json parse error: %v", ledger.ErrValidation, err))
		return
	}

	res, err := s.ledger.PostVerification("API User", req)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}

func (s *Server) handleLockPeriod(w http.ResponseWriter, r *http.Request) {
	var req struct {
		YearMonth string `json:"year_month"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ledger.ErrValidation)
		return
	}

	res, err := s.ledger.LockPeriod(req.YearMonth, "API User")
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (s *Server) handleGetFiscalYears(w http.ResponseWriter, r *http.Request) {
	years, err := s.ledger.GetFiscalYears()
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(years)
}

func (s *Server) handleCreateFiscalYear(w http.ResponseWriter, r *http.Request) {
	fy, err := s.ledger.CreateFiscalYear("API User")
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(fy)
}

func (s *Server) handleLockFiscalYear(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, fmt.Errorf("%w: invalid fiscal year ID", ledger.ErrValidation))
		return
	}

	res, err := s.ledger.LockFiscalYear(id, "API User")
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (s *Server) handleGetVatReport(w http.ResponseWriter, r *http.Request) {
	start := r.URL.Query().Get("start_date")
	end := r.URL.Query().Get("end_date")

	if start == "" || end == "" {
		writeError(w, fmt.Errorf("start_date and end_date required"))
		return
	}

	report, err := s.ledger.GetVatReport(start, end)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

func (s *Server) handleTransferVat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("invalid request body: %w", err))
		return
	}

	if req.StartDate == "" || req.EndDate == "" {
		writeError(w, fmt.Errorf("start_date and end_date required"))
		return
	}

	user := "API User" // Replace with real auth user if applicable

	err := s.ledger.TransferVat(user, req.StartDate, req.EndDate)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (s *Server) handleVerifyChain(w http.ResponseWriter, r *http.Request) {
	valid, err := s.ledger.VerifyChain()
	if err != nil {
		writeError(w, err)
		return
	}
	if !valid {
		writeError(w, ledger.ErrWormViolation)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"message": "WORM chain is intact and valid.",
	})
}

func (s *Server) handleStornoVerification(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, fmt.Errorf("%w: invalid verification ID", ledger.ErrValidation))
		return
	}

	res, err := s.ledger.StornoVerification(id, "API User")
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}

func (s *Server) handleVoidDraftVerification(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, fmt.Errorf("%w: invalid verification ID", ledger.ErrValidation))
		return
	}

	err = s.ledger.VoidDraftVerification(id, "API User")
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"message": "Draft verification voided successfully",
	})
}

func (s *Server) handleFillGaps(w http.ResponseWriter, r *http.Request) {
	err := s.ledger.FillSequenceGaps("API User")
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"message": "Sequence gaps filled successfully",
	})
}

func (s *Server) handleSealVerifications(w http.ResponseWriter, r *http.Request) {
	res, err := s.ledger.SealVerifications("API User", false)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

func (s *Server) handleGetAttachment(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	if hash == "" {
		writeError(w, fmt.Errorf("%w: hash is required", ledger.ErrValidation))
		return
	}

	if matched, _ := regexp.MatchString("^[a-f0-9]{64}$", hash); !matched {
		writeError(w, fmt.Errorf("%w: invalid hash format", ledger.ErrValidation))
		return
	}

	mimeType, filePath, err := s.ledger.GetAttachmentInfo(hash)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Disposition", "inline; filename=\"receipt_"+hash[:8]+"\"")
	http.ServeFile(w, r, filePath)
}

func (s *Server) handleGetSamlingsplan(w http.ResponseWriter, r *http.Request) {
	html, err := s.ledger.GenerateSamlingsplan()
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

func (s *Server) handleGetAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.ledger.GetAccounts()
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(accounts)
}

func (s *Server) handleGetReports(w http.ResponseWriter, r *http.Request) {
	yearIDStr := r.URL.Query().Get("year_id")
	var yearID *int64
	if yearIDStr != "" {
		if id, err := strconv.ParseInt(yearIDStr, 10, 64); err == nil {
			yearID = &id
		}
	}

	report, err := s.ledger.GetFinancialReport(yearID)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Kunde inte generera rapport: %v", err)))
		return
	}

	funcMap := template.FuncMap{
		"money": func(v int64) string {
			return fmt.Sprintf("%.2f", float64(v)/100.0)
		},
	}

	tmpl, err := template.New("reports.html").Funcs(funcMap).ParseFiles("frontend/views/reports.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Kunde inte ladda mall: %v", err)))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, report); err != nil {
		fmt.Printf("Template execute error: %v\n", err)
	}
}

// handleAddAccount hanterar skapandet av ett nytt konto.
func (s *Server) handleAddAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
		Name string `json:"name"`
		Type string `json:"type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("invalid request body: %w", err))
		return
	}

	if err := s.ledger.AddAccount(req.Code, req.Name, req.Type); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"status":"account created"}`))
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.ledger.GetSettings()
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

func (s *Server) handlePostSettings(w http.ResponseWriter, r *http.Request) {
	var req models.CompanySettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ledger.ErrValidation)
		return
	}

	if err := s.ledger.UpdateSettings(req); err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(req)
}

func (s *Server) handleExportSIE4(w http.ResponseWriter, r *http.Request) {
	activeYear, err := s.ledger.GetActiveFiscalYear()
	if err != nil {
		writeError(w, err)
		return
	}

	sieData, err := s.ledger.GenerateSIE4(activeYear.ID)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Kunde inte exportera SIE-4: %v", err)))
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\"export.se\"")
	w.WriteHeader(http.StatusOK)
	w.Write(sieData)
}
