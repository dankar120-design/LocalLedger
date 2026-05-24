package api

import (
	"archive/zip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/pbkdf2"

	"localledger/internal/ledger"
)

const cryptSignature = "LLCRYPT\x01"

func deriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, 10000, 32, sha256.New)
}

func encryptPayload(plainData []byte, password string) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	key := deriveKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create gcm: %w", err)
	}

	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aesgcm.Seal(nil, nonce, plainData, nil)

	var output []byte
	output = append(output, []byte(cryptSignature)...)
	output = append(output, salt...)
	output = append(output, nonce...)
	output = append(output, ciphertext...)

	return output, nil
}

func decryptPayload(encryptedData []byte, password string) ([]byte, error) {
	if len(encryptedData) < 8+16+12 {
		return nil, fmt.Errorf("invalid encrypted data: too short")
	}

	sig := string(encryptedData[:8])
	if sig != cryptSignature {
		return nil, fmt.Errorf("invalid signature")
	}

	salt := encryptedData[8:24]
	nonce := encryptedData[24:36]
	ciphertext := encryptedData[36:]

	key := deriveKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create gcm: %w", err)
	}

	plainData, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data (wrong password?)")
	}

	return plainData, nil
}

func (s *Server) handleExportBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	password := req.Password

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

	if err := s.ledger.ExportSnapshot(tempPath); err != nil {
		log.Printf("Failed to create snapshot: %v", err)
		http.Error(w, "Failed to create database snapshot", http.StatusInternalServerError)
		return
	}

	// Skapa en temporär zip-fil
	tempZipFile, err := os.CreateTemp("", "localledger_backup_*.zip")
	if err != nil {
		log.Printf("Failed to create temp zip file: %v", err)
		http.Error(w, "Failed to prepare zip file", http.StatusInternalServerError)
		return
	}
	tempZipPath := tempZipFile.Name()
	defer os.Remove(tempZipPath) // Städa alltid upp!

	// Skapa zip-skrivaren
	zw := zip.NewWriter(tempZipFile)

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

	// Lägg till ledger.db (från snapshoten)
	if err := addFileToZip("ledger.db", tempPath); err != nil {
		log.Printf("Failed to zip database: %v", err)
		zw.Close()
		tempZipFile.Close()
		http.Error(w, "Failed to zip database", http.StatusInternalServerError)
		return
	}

	// Lägg till alla bilagor
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
	}
	// Lägg till logotyp om en sådan finns konfigurerad och existerar på disk
	settings, err := s.ledger.GetSettings()
	if err == nil && settings.LogoPath != "" {
		logoPath := filepath.Join(s.ledger.WorkspacePath(), settings.LogoPath)
		if _, errStat := os.Stat(logoPath); errStat == nil {
			if errZip := addFileToZip(settings.LogoPath, logoPath); errZip != nil {
				log.Printf("Failed to zip logo %s: %v", settings.LogoPath, errZip)
			}
		}
	}

	// Stäng zip-skrivaren och filen för att spara allt
	if err := zw.Close(); err != nil {
		log.Printf("Failed to close zip writer: %v", err)
		tempZipFile.Close()
		http.Error(w, "Failed to close zip writer", http.StatusInternalServerError)
		return
	}
	tempZipFile.Close()

	var outputBytes []byte
	if password != "" {
		zipBytes, err := os.ReadFile(tempZipPath)
		if err != nil {
			log.Printf("Failed to read temp zip: %v", err)
			http.Error(w, "Failed to read zip", http.StatusInternalServerError)
			return
		}

		encryptedBytes, err := encryptPayload(zipBytes, password)
		if err != nil {
			log.Printf("Failed to encrypt zip payload: %v", err)
			http.Error(w, "Failed to encrypt backup", http.StatusInternalServerError)
			return
		}
		outputBytes = encryptedBytes
	} else {
		zipBytes, err := os.ReadFile(tempZipPath)
		if err != nil {
			log.Printf("Failed to read temp zip: %v", err)
			http.Error(w, "Failed to read zip", http.StatusInternalServerError)
			return
		}
		outputBytes = zipBytes
	}

	// Förbered HTTP-svar
	var filename string
	if password != "" {
		filename = fmt.Sprintf("LocalLedger_Backup_%s.zip.enc", time.Now().Format("20060102_150405"))
		w.Header().Set("Content-Type", "application/octet-stream")
	} else {
		filename = fmt.Sprintf("LocalLedger_Backup_%s.zip", time.Now().Format("20060102_150405"))
		w.Header().Set("Content-Type", "application/zip")
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(outputBytes)))

	w.Write(outputBytes)
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

	// Kontrollera om filen är krypterad
	sigBuf := make([]byte, 8)
	fCheck, err := os.Open(tempZip.Name())
	if err != nil {
		http.Error(w, "Serverfel", http.StatusInternalServerError)
		return
	}
	n, err := fCheck.Read(sigBuf)
	fCheck.Close()

	var finalZipPath string
	if err == nil && n == 8 && string(sigBuf) == cryptSignature {
		password := r.FormValue("password")
		if password == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": "password_required"}`))
			return
		}

		encryptedBytes, err := os.ReadFile(tempZip.Name())
		if err != nil {
			http.Error(w, "Kunde inte läsa uppladdad fil", http.StatusInternalServerError)
			return
		}

		decryptedBytes, err := decryptPayload(encryptedBytes, password)
		if err != nil {
			log.Printf("Decryption failed: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": "invalid_password"}`))
			return
		}

		decryptedZip, err := os.CreateTemp("", "decrypted_*.zip")
		if err != nil {
			http.Error(w, "Serverfel", http.StatusInternalServerError)
			return
		}
		defer os.Remove(decryptedZip.Name())

		if _, err := decryptedZip.Write(decryptedBytes); err != nil {
			decryptedZip.Close()
			http.Error(w, "Serverfel", http.StatusInternalServerError)
			return
		}
		decryptedZip.Close()
		finalZipPath = decryptedZip.Name()
	} else {
		finalZipPath = tempZip.Name()
	}

	// 2. Skapa en "Staging"-mapp inuti workspace för att garantera att os.Rename är atomär (samma volym)
	stagingDir, err := os.MkdirTemp(s.ledger.WorkspacePath(), ".restore_staging_*")
	if err != nil {
		http.Error(w, "Serverfel", http.StatusInternalServerError)
		return
	}

	// Skapa attachments i staging
	os.MkdirAll(filepath.Join(stagingDir, "attachments"), 0755)

	// 3. Packa upp säkert (Anti-Zip Slip)
	zipReader, err := zip.OpenReader(finalZipPath)
	if err != nil {
		os.RemoveAll(stagingDir)
		http.Error(w, "Ogiltig eller korrupt zip-fil", http.StatusBadRequest)
		return
	}
	defer zipReader.Close()

	hasLedgerDB := false
	for _, f := range zipReader.File {
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
	tempLedger, err := ledger.OpenLedger(stagingDir, "v3.0.0")
	if err != nil {
		log.Printf("Restore validation failed: %v", err)
		os.RemoveAll(stagingDir)
		http.Error(w, fmt.Sprintf("Säkerhetskopian är ogiltig eller från en inkompatibel version: %v", err), http.StatusBadRequest)
		return
	}
	tempLedger.Close()

	// 5. Point of No Return: Skicka OK till webbläsaren
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok"}`))

	// 6. Asynkron Drop & Exit
	go func() {
		time.Sleep(1 * time.Second)

		log.Println("INITIATING DROP & EXIT RESTORE...")

		s.ledger.Close()

		wsPath := s.ledger.WorkspacePath()
		dbPath := filepath.Join(wsPath, "ledger.db")
		walPath := filepath.Join(wsPath, "ledger.db-wal")
		shmPath := filepath.Join(wsPath, "ledger.db-shm")
		attachPath := filepath.Join(wsPath, "attachments")

		os.Remove(walPath)
		os.Remove(shmPath)

		// Robust retry-loop för borttagning av gamla ledger.db på Windows (fil-lås)
		var removeErr error
		for i := 0; i < 5; i++ {
			removeErr = os.Remove(dbPath)
			if removeErr == nil || os.IsNotExist(removeErr) {
				break
			}
			log.Printf("[Hot Restore] Database file is locked. Retrying remove in 200ms... (%d/5): %v", i+1, removeErr)
			time.Sleep(200 * time.Millisecond)
		}
		if removeErr != nil && !os.IsNotExist(removeErr) {
			log.Printf("[CRITICAL ERROR] Failed to remove locked database file during hot restore: %v", removeErr)
			os.RemoveAll(stagingDir)
			return
		}

		// Robust retry-loop för att flytta nya ledger.db på plats
		var renameErr error
		for i := 0; i < 5; i++ {
			renameErr = os.Rename(filepath.Join(stagingDir, "ledger.db"), dbPath)
			if renameErr == nil {
				break
			}
			log.Printf("[Hot Restore] Database file is locked. Retrying rename in 200ms... (%d/5): %v", i+1, renameErr)
			time.Sleep(200 * time.Millisecond)
		}
		if renameErr != nil {
			log.Printf("[CRITICAL ERROR] Failed to rename database file during hot restore: %v", renameErr)
			os.RemoveAll(stagingDir)
			return
		}

		// Robust retry-loop för attachments-mappen
		var attachRemoveErr error
		for i := 0; i < 5; i++ {
			attachRemoveErr = os.RemoveAll(attachPath)
			if attachRemoveErr == nil {
				break
			}
			log.Printf("[Hot Restore] Attachments folder is locked. Retrying remove in 200ms... (%d/5): %v", i+1, attachRemoveErr)
			time.Sleep(200 * time.Millisecond)
		}

		var attachRenameErr error
		for i := 0; i < 5; i++ {
			attachRenameErr = os.Rename(filepath.Join(stagingDir, "attachments"), attachPath)
			if attachRenameErr == nil {
				break
			}
			log.Printf("[Hot Restore] Attachments folder is locked. Retrying rename in 200ms... (%d/5): %v", i+1, attachRenameErr)
			time.Sleep(200 * time.Millisecond)
		}

		// Flytta eventuella andra filer (som company_logo.*) från staging till workspace
		if files, errRead := os.ReadDir(stagingDir); errRead == nil {
			for _, f := range files {
				if !f.IsDir() && f.Name() != "ledger.db" {
					dest := filepath.Join(wsPath, f.Name())
					os.Remove(dest)
					os.Rename(filepath.Join(stagingDir, f.Name()), dest)
				}
			}
		}

		os.RemoveAll(stagingDir)

		log.Println("RESTORE COMPLETE. SHUTTING DOWN LOCALLEDGER.")
		os.Exit(0)
	}()
}
