package api

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"localledger/internal/reports"
)

// handleExportSRU genererar INFO.SRU och BLANKETTER.SRU och paketerar dem i en nedladdningsbar ZIP-fil.
func (s *Server) handleExportSRU(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	yearIDStr := r.URL.Query().Get("year_id")
	var yearID int64
	var err error

	if yearIDStr != "" {
		yearID, err = strconv.ParseInt(yearIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Ogiltigt year_id", http.StatusBadRequest)
			return
		}
	} else {
		activeYear, err := s.ledger.GetActiveFiscalYear()
		if err != nil {
			writeError(w, err)
			return
		}
		yearID = activeYear.ID
	}

	infoBytes, blanketterBytes, err := reports.GenerateSRUFiles(s.ledger, yearID)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Kunde inte generera SRU-filer: %v", err)))
		return
	}

	// Skapa ZIP-arkiv i minnet
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)

	infoFile, err := zw.Create("INFO.SRU")
	if err != nil {
		http.Error(w, "Misslyckades att paketera SRU", http.StatusInternalServerError)
		return
	}
	if _, err := infoFile.Write(infoBytes); err != nil {
		http.Error(w, "Misslyckades att skriva INFO.SRU", http.StatusInternalServerError)
		return
	}

	blanketterFile, err := zw.Create("BLANKETTER.SRU")
	if err != nil {
		http.Error(w, "Misslyckades att paketera SRU", http.StatusInternalServerError)
		return
	}
	if _, err := blanketterFile.Write(blanketterBytes); err != nil {
		http.Error(w, "Misslyckades att skriva BLANKETTER.SRU", http.StatusInternalServerError)
		return
	}

	if err := zw.Close(); err != nil {
		http.Error(w, "Misslyckades att stänga ZIP-arkiv", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("LocalLedger_SRU_%s.zip", time.Now().Format("20060102"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", zipBuf.Len()))
	w.WriteHeader(http.StatusOK)
	w.Write(zipBuf.Bytes())
}
