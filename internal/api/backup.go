package api

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func (s *Server) handleExportBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Skapa en temporär databasfil för VACUUM INTO
	tempFile, err := os.CreateTemp("", "localledger_backup_*.db")
	if err != nil {
		log.Printf("Failed to create temp file: %v", err)
		http.Error(w, "Failed to prepare backup", http.StatusInternalServerError)
		return
	}
	tempPath := tempFile.Name()
	tempFile.Close()            // SQLite måste öppna den, så stäng vår filpekare
	defer os.Remove(tempPath) // Städa alltid upp!

	// 2. Ta en snapshot via VACUUM INTO
	// Detta garanterar en 100% konsekvent backup även i WAL mode
	// OBS: VACUUM INTO körs mot den *aktiva* databasen via l.db.Exec() för att kopiera DEN, 
	// men eftersom den metoden inte är exponerad måste vi lägga den i ledger.go.
	// Ah, vi kan skapa ExportBackup() i ledger.go som sköter db-anropet!
	
	if err := s.ledger.ExportSnapshot(tempPath); err != nil {
		log.Printf("Failed to create snapshot: %v", err)
		http.Error(w, "Failed to create database snapshot", http.StatusInternalServerError)
		return
	}

	// 3. Förbered HTTP-svar för zip-fil
	filename := fmt.Sprintf("LocalLedger_Backup_%s.zip", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	// 4. Skapa zip-skrivaren och koppla den direkt till HTTP ResponseWriter
	zw := zip.NewWriter(w)
	defer zw.Close()

	// Hjälpfunktion för att zippa en fil
	addFileToZip := func(zipPath, sourcePath string) error {
		sourceFile, err := os.Open(sourcePath)
		if err != nil {
			return err
		}
		defer sourceFile.Close()

		writer, err := zw.Create(zipPath)
		if err != nil {
			return err
		}

		_, err = io.Copy(writer, sourceFile)
		return err
	}

	// 5. Lägg till ledger.db (från snapshoten)
	if err := addFileToZip("ledger.db", tempPath); err != nil {
		log.Printf("Failed to zip database: %v", err)
		return
	}

	// 6. Lägg till alla bilagor
	attachmentsDir := filepath.Join(s.ledger.WorkspacePath(), "attachments")
	entries, err := os.ReadDir(attachmentsDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				sourcePath := filepath.Join(attachmentsDir, entry.Name())
				zipPath := "attachments/" + entry.Name()
				if err := addFileToZip(zipPath, sourcePath); err != nil {
					log.Printf("Failed to zip attachment %s: %v", entry.Name(), err)
					// Vi ignorerar enskilda fil-fel för att inte avbryta hela backupen
				}
			}
		}
	} else {
		log.Printf("Attachments directory not found or error reading: %v", err)
	}
}
