package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"net/http"
	"regexp"
	"strconv"

	"localledger/internal/ledger"
	"localledger/internal/models"
	"localledger/internal/ocr"

	"github.com/xuri/excelize/v2"
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
	mux.HandleFunc("GET /api/reports/financial", s.handleGetFinancialJSON)
	mux.HandleFunc("GET /api/reports/excel", s.handleGetFinancialExcel)
	mux.HandleFunc("GET /api/dashboard", s.handleGetDashboard)
	mux.HandleFunc("GET /api/accounts", s.handleGetAccounts)
	mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	mux.HandleFunc("POST /api/settings", s.handlePostSettings)
	mux.HandleFunc("POST /api/settings/shortcut", s.handlePostShortcut)
	mux.HandleFunc("POST /api/settings/logo", s.handleUploadLogo)
	mux.HandleFunc("GET /api/settings/logo", s.handleServeLogo)
	mux.HandleFunc("POST /api/accounts", s.handleAddAccount)
	mux.HandleFunc("GET /api/export/sie4", s.handleExportSIE4)
	mux.HandleFunc("GET /api/export/sru", s.handleExportSRU)
	mux.HandleFunc("GET /api/reconciliation/match", s.handleGetReconciliationMatches)
	mux.HandleFunc("POST /api/export/backup", s.handleExportBackup)
	mux.HandleFunc("POST /api/import/backup", s.handleRestoreBackup)
	mux.HandleFunc("GET /api/vat-report", s.handleGetVatReport)
	mux.HandleFunc("POST /api/vat-report/transfer", s.handleTransferVat)

	// Invoices
	mux.HandleFunc("GET /api/invoices", s.HandleGetInvoices)
	mux.HandleFunc("POST /api/invoices", s.HandleCreateInvoice)
	mux.HandleFunc("GET /api/invoices/{id}", s.HandleGetInvoice)
	mux.HandleFunc("PUT /api/invoices/{id}", s.HandleUpdateInvoice)
	mux.HandleFunc("DELETE /api/invoices/{id}", s.HandleDeleteInvoice)
	mux.HandleFunc("POST /api/invoices/{id}/post", s.HandlePostInvoice)
	mux.HandleFunc("POST /api/invoices/{id}/pay", s.HandlePayInvoice)
	mux.HandleFunc("GET /api/invoices/{id}/pdf", s.HandleGetInvoicePDF)
	mux.HandleFunc("POST /api/invoices/{id}/credit", s.HandleCreditInvoice)
	mux.HandleFunc("POST /api/invoices/{id}/settle", s.HandleSettleInvoice)

	mux.HandleFunc("POST /api/import/sie4", s.handleImportSIE4)
	mux.HandleFunc("POST /api/fiscal-years/{id}/generate-ib", s.handleGenerateIB)
	mux.HandleFunc("POST /api/ocr/parse", s.handleParseOCR)

	// Inbox Routes
	mux.HandleFunc("GET /api/inbox", s.handleGetInbox)
	mux.HandleFunc("GET /api/inbox/{id}/download", s.handleDownloadInbox)
	mux.HandleFunc("POST /api/inbox/upload", s.handleUploadInbox)
	mux.HandleFunc("DELETE /api/inbox/{id}", s.handleDeleteInbox)
	// Customers
	mux.HandleFunc("GET /api/customers", s.handleGetCustomers)
	mux.HandleFunc("DELETE /api/customers/{id}", s.handleAnonymizeCustomer)
	mux.HandleFunc("POST /api/inbox/fetch-cloud", s.handleFetchCloud)
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

func (s *Server) handleGetDashboard(w http.ResponseWriter, r *http.Request) {
	yearIDStr := r.URL.Query().Get("year_id")
	var yearID *int64
	if yearIDStr != "" {
		if id, err := strconv.ParseInt(yearIDStr, 10, 64); err == nil {
			yearID = &id
		}
	}

	metrics, err := s.ledger.GetDashboardMetrics(yearID)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
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
	res, err := s.ledger.SealVerifications("API User", true)
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

func (s *Server) handleGetFinancialJSON(w http.ResponseWriter, r *http.Request) {
	yearIDStr := r.URL.Query().Get("year_id")
	var yearID *int64
	if yearIDStr != "" {
		if id, err := strconv.ParseInt(yearIDStr, 10, 64); err == nil {
			yearID = &id
		}
	}

	report, err := s.ledger.GetFinancialReport(yearID)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

func (s *Server) handleGetFinancialExcel(w http.ResponseWriter, r *http.Request) {
	yearIDStr := r.URL.Query().Get("year_id")
	var yearID *int64
	if yearIDStr != "" {
		if id, err := strconv.ParseInt(yearIDStr, 10, 64); err == nil {
			yearID = &id
		}
	}

	report, err := s.ledger.GetFinancialReport(yearID)
	if err != nil {
		writeError(w, err)
		return
	}

	f := excelize.NewFile()
	defer f.Close()

	sheet := "Finansiell Rapport"
	f.SetSheetName("Sheet1", sheet)

	// Kolumnbredder
	f.SetColWidth(sheet, "A", "A", 15)
	f.SetColWidth(sheet, "B", "B", 40)
	f.SetColWidth(sheet, "C", "C", 20)

	// Styles
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14},
	})
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "#FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#3B82F6"}, Pattern: 1},
	})
	moneyFormat := "#,##0.00 \"kr\""
	moneyStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: &moneyFormat,
	})
	boldMoneyStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		CustomNumFmt: &moneyFormat,
	})
	boldStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})

	row := 1
	f.SetCellValue(sheet, fmt.Sprintf("A%d", row), "Finansiell Rapport")
	f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), titleStyle)
	row++
	f.SetCellValue(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("Räkenskapsår: %s", report.FiscalYear))
	row += 2

	writeHeader := func(title string) {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), title)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("C%d", row), headerStyle)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), "Beskrivning")
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), "Belopp (SEK)")
		row++
	}

	writeRow := func(code, name string, balance int64) {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), code)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), name)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), float64(balance)/100.0)
		f.SetCellStyle(sheet, fmt.Sprintf("C%d", row), fmt.Sprintf("C%d", row), moneyStyle)
		row++
	}

	writeTotal := func(title string, total int64) {
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), title)
		f.SetCellStyle(sheet, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), boldStyle)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), float64(total)/100.0)
		f.SetCellStyle(sheet, fmt.Sprintf("C%d", row), fmt.Sprintf("C%d", row), boldMoneyStyle)
		row++
	}

	// Resultaträkning
	writeHeader("Intäkter")
	for _, acc := range report.Income {
		writeRow(acc.AccountCode, acc.AccountName, acc.Balance)
	}
	writeTotal("Summa Intäkter:", report.TotalIncome)
	row++

	writeHeader("Kostnader")
	for _, acc := range report.Expenses {
		writeRow(acc.AccountCode, acc.AccountName, acc.Balance)
	}
	writeTotal("Summa Kostnader:", report.TotalExpenses)
	row++

	writeTotal("Årets Resultat:", report.NetIncome)
	row += 2

	// Balansräkning
	writeHeader("Tillgångar")
	for _, acc := range report.Assets {
		writeRow(acc.AccountCode, acc.AccountName, acc.Balance)
	}
	writeTotal("Summa Tillgångar:", report.TotalAssets)
	row++

	writeHeader("Skulder & Eget Kapital")
	for _, acc := range report.Liabilities {
		writeRow(acc.AccountCode, acc.AccountName, acc.Balance)
	}
	writeTotal("Summa Skulder:", report.TotalLiabilities)
	writeTotal("Årets Resultat:", report.NetIncome)
	writeTotal("Summa Skulder & Eget Kapital:", report.CalculatedEquity)

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"LocalLedger_Rapport_%s.xlsx\"", report.FiscalYear))
	f.WriteTo(w)
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

	if !s.isSandbox {
		if err := SaveGlobalConfig(s.ledger.WorkspacePath(), req.Name); err != nil {
			log.Printf("Varning: Kunde inte uppdatera global config efter settings-ändring: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(req)
}

func (s *Server) handlePostShortcut(w http.ResponseWriter, r *http.Request) {
	if s.isSandbox {
		writeError(w, fmt.Errorf("genvägar kan inte skapas i Sandbox-läge"))
		return
	}

	companyName := s.CompanyName()
	workspacePath := s.ledger.WorkspacePath()

	if err := createDesktopShortcut(workspacePath, companyName); err != nil {
		writeError(w, fmt.Errorf("misslyckades att skapa skrivbordsgenväg: %w", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"success"}`))
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

func (s *Server) handleImportSIE4(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 20*1024*1024)

	user := "API User"

	// Läs yearID från form-data eller query
	yearIDStr := r.URL.Query().Get("yearID")
	if yearIDStr == "" {
		yearIDStr = r.FormValue("yearID")
	}

	yearID, err := strconv.ParseInt(yearIDStr, 10, 64)
	if err != nil {
		writeError(w, fmt.Errorf("%w: Ogiltigt yearID", ledger.ErrValidation))
		return
	}

	err = r.ParseMultipartForm(10 << 20) // 10 MB max memory
	if err != nil {
		writeError(w, fmt.Errorf("%w: failed to parse multipart form: %v", ledger.ErrValidation, err))
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, fmt.Errorf("%w: missing file field", ledger.ErrValidation))
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		writeError(w, fmt.Errorf("failed to read file: %w", err))
		return
	}

	dryRunStr := r.URL.Query().Get("dry_run")
	if dryRunStr == "" {
		dryRunStr = r.FormValue("dry_run")
	}
	isDryRun := (dryRunStr == "true")

	if isDryRun {
		preview, err := s.ledger.PreviewSIE4(yearID, fileBytes)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(preview)
		return
	}

	if err := s.ledger.ImportSIE4(user, yearID, fileBytes); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

func (s *Server) handleGenerateIB(w http.ResponseWriter, r *http.Request) {
	user := "API User"

	idStr := r.PathValue("id")
	toYearID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, fmt.Errorf("%w: ogiltigt toYearID", ledger.ErrValidation))
		return
	}

	var req struct {
		FromYearID int64 `json:"from_year_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: ogiltig JSON", ledger.ErrValidation))
		return
	}

	if err := s.ledger.GenerateOpeningBalance(user, req.FromYearID, toYearID); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

func (s *Server) handleParseOCR(w http.ResponseWriter, r *http.Request) {
	// Begränsa payload till ca 10KB för att förhindra DOS/ReDoS från hallucinerad OCR-text
	r.Body = http.MaxBytesReader(w, r.Body, 10*1024)

	var req struct {
		RawText string `json:"raw_text"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("invalid request body: %w", err))
		return
	}

	// Hämta historiska leverantörer för Inverted Matching
	knownVendors, err := s.ledger.GetDistinctVendors()
	if err != nil {
		// Vi loggar felet men avbryter inte, parsern kan köras utan knownVendors
		fmt.Printf("Warning: failed to fetch distinct vendors: %v\n", err)
		knownVendors = []string{}
	}

	res := ocr.ParseOCRText(req.RawText, knownVendors)
	if res.Vendor != "" {
		if suggested, err := s.ledger.GetSuggestedAccountForVendor(res.Vendor); err == nil {
			res.SuggestedAccount = suggested
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

func (s *Server) handleGetInbox(w http.ResponseWriter, r *http.Request) {
	items, err := s.inbox.GetAllItems()
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []models.InboxItem{} // Prevent null in JSON
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func (s *Server) handleUploadInbox(w http.ResponseWriter, r *http.Request) {
	// Max 20 MB (lite extra marginal)
	r.Body = http.MaxBytesReader(w, r.Body, 21*1024*1024)
	if err := r.ParseMultipartForm(20 * 1024 * 1024); err != nil {
		writeError(w, fmt.Errorf("file too large or invalid multipart: %w", err))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, fmt.Errorf("missing file field: %w", err))
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, fmt.Errorf("failed to read file: %w", err))
		return
	}

	item, err := s.inbox.SaveFile(data, header.Filename, "local")
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item)
}

func (s *Server) handleDeleteInbox(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.inbox.DeleteItem(id); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"deleted"}`))
}

func (s *Server) handleDownloadInbox(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mimeType, filePath, err := s.inbox.GetItemInfo(id)
	if err != nil {
		writeError(w, err)
		return
	}
	
	file, err := os.Open(filePath)
	if err != nil {
		writeError(w, err)
		return
	}
	defer file.Close()

	if info, err := file.Stat(); err == nil {
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Cache-Control", "no-cache")
	io.Copy(w, file)
}

func (s *Server) handleFetchCloud(w http.ResponseWriter, r *http.Request) {
	result, err := s.inbox.FetchFromCloud()
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"fetched": result.Fetched,
		"failed":  result.Failed,
		"errors":  result.Errors,
	})
}

func (s *Server) handleGetCustomers(w http.ResponseWriter, r *http.Request) {
	customers, err := s.ledger.GetAllCustomers()
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if customers == nil {
		customers = []models.Customer{}
	}
	json.NewEncoder(w).Encode(customers)
}

func (s *Server) handleAnonymizeCustomer(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, fmt.Errorf("invalid id"))
		return
	}

	if err := s.ledger.AnonymizeCustomer(id); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
