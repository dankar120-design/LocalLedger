package api

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"localledger/internal/ledger"
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

func (s *Server) handleRestoreBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Läs zip-filen från formuläret
	r.ParseMultipartForm(50 << 20) // 50 MB
	file, _, err := r.FormFile("backup_zip")
	if err != nil {
		http.Error(w, "Ingen fil uppladdad", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Spara uppladdad fil temporärt
	tempZip, err := os.CreateTemp("", "upload_*.zip")
	if err != nil {
		http.Error(w, "Serverfel", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempZip.Name())

	if _, err := io.Copy(tempZip, file); err != nil {
		http.Error(w, "Kunde inte spara uppladdad fil", http.StatusInternalServerError)
		return
	}
	tempZip.Close()

	// 2. Skapa en "Staging"-mapp inuti workspace för att garantera att os.Rename är atomär (samma volym)
	stagingDir, err := os.MkdirTemp(s.ledger.WorkspacePath(), ".restore_staging_*")
	if err != nil {
		http.Error(w, "Serverfel", http.StatusInternalServerError)
		return
	}
	// OBS: Vi använder INTE defer os.RemoveAll(stagingDir) här, eftersom vi asynkront väntar på att använda den.

	// Skapa attachments i staging
	os.MkdirAll(filepath.Join(stagingDir, "attachments"), 0755)

	// 3. Packa upp säkert (Anti-Zip Slip)
	zipReader, err := zip.OpenReader(tempZip.Name())
	if err != nil {
		os.RemoveAll(stagingDir)
		http.Error(w, "Ogiltig zip-fil", http.StatusBadRequest)
		return
	}
	defer zipReader.Close()

	hasLedgerDB := false
	for _, f := range zipReader.File {
		// Anti Zip-Slip
		cleanedName := filepath.Clean(f.Name)
		if strings.Contains(cleanedName, "..") || filepath.IsAbs(cleanedName) {
			continue // Ignorera farliga sökvägar
		}

		if cleanedName == "ledger.db" {
			hasLedgerDB = true
		}

		targetPath := filepath.Join(stagingDir, cleanedName)
		if f.FileInfo().IsDir() {
			os.MkdirAll(targetPath, f.Mode())
			continue
		}

		os.MkdirAll(filepath.Dir(targetPath), 0755)
		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			os.RemoveAll(stagingDir)
			http.Error(w, "Kunde inte packa upp fil", http.StatusInternalServerError)
			return
		}
		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			os.RemoveAll(stagingDir)
			http.Error(w, "Kunde inte läsa fil ur zip", http.StatusInternalServerError)
			return
		}
		io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
	}

	if !hasLedgerDB {
		os.RemoveAll(stagingDir)
		http.Error(w, "Ogiltig säkerhetskopia: ledger.db saknas", http.StatusBadRequest)
		return
	}

	// 4. Validera den uppackade databasen (Fångar Downgrades & Korruption)
	tempLedger, err := ledger.OpenLedger(stagingDir, "v1.4.0")
	if err != nil {
		log.Printf("Restore validation failed: %v", err)
		os.RemoveAll(stagingDir)
		http.Error(w, fmt.Sprintf("Säkerhetskopian är ogiltig eller från en inkompatibel version: %v", err), http.StatusBadRequest)
		return
	}
	// Validering lyckades, stäng den så vi släpper låsen
	tempLedger.Close()

	// 5. Point of No Return: Skicka OK till webbläsaren
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok"}`))
	
	// 6. Asynkron Drop & Exit
	go func() {
		// Ge webbläsaren 1 sekund att ta emot HTTP-svaret
		time.Sleep(1 * time.Second)

		log.Println("INITIATING DROP & EXIT RESTORE...")
		
		// 6.1 Stäng aktiv DB
		s.ledger.Close()

		wsPath := s.ledger.WorkspacePath()
		dbPath := filepath.Join(wsPath, "ledger.db")
		walPath := filepath.Join(wsPath, "ledger.db-wal")
		shmPath := filepath.Join(wsPath, "ledger.db-shm")
		attachPath := filepath.Join(wsPath, "attachments")

		// 6.2 Ta bort -wal och -shm för att undvika korruption
		os.Remove(walPath)
		os.Remove(shmPath)

		// 6.3 Byt ut ledger.db
		os.Remove(dbPath)
		os.Rename(filepath.Join(stagingDir, "ledger.db"), dbPath)

		// 6.4 Byt ut attachments
		os.RemoveAll(attachPath)
		os.Rename(filepath.Join(stagingDir, "attachments"), attachPath)

		// 6.5 Städa explicit före exit, eftersom os.Exit() inte kör defer-anrop
		os.RemoveAll(stagingDir)

		log.Println("RESTORE COMPLETE. SHUTTING DOWN LOCALLEDGER.")
		os.Exit(0)
	}()
}
