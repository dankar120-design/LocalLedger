package api

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"localledger/frontend"
	"localledger/internal/ledger"
)

// SetupRequest mappar mot JSON payload från klienten
type SetupRequest struct {
	Action string `json:"action"` // "new_workspace" eller "sandbox"
}

// StartSetupServer startar on-boarding UI:t.
// Returnerar vald workspace-mapp (eller "__SANDBOX__"), samt ett eventuellt fel.
func StartSetupServer(port int) (string, error) {
	mux := http.NewServeMux()
	
	// Kanal för att kommunicera resultatet tillbaka till main thread
	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return "", fmt.Errorf("failed to bind setup port %d: %w", port, err)
	}

	srv := &http.Server{
		Handler: mux,
	}

	// Skapa en context för att garantera att heartbeat stängs säkert
	ctxHeartbeat, cancelHeartbeat := context.WithCancel(context.Background())
	defer cancelHeartbeat()

	// Heartbeat-mekanism mot Zombie-processer
	var lastPing time.Time
	var pingMu sync.Mutex
	lastPing = time.Now()

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		pingMu.Lock()
		lastPing = time.Now()
		pingMu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/" {
			content, err := frontend.FS.ReadFile("views/setup.html")
			if err != nil {
				http.Error(w, "Setup-fil saknas i binären", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(content)
			return
		}

		if r.Method == http.MethodPost && r.URL.Path == "/setup" {
			var req SetupRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error": "Ogiltig request"}`, http.StatusBadRequest)
				return
			}

			var selectedWorkspace string

			if req.Action == "sandbox" {
				selectedWorkspace = "__SANDBOX__"
			} else if req.Action == "new_workspace" {
				// Skapa en mapp-dialog via PowerShell
				psScript := `
				Add-Type -AssemblyName System.windows.forms
				$f = New-Object System.Windows.Forms.FolderBrowserDialog
				$f.Description = "Välj var du vill spara din LocalLedger bokföring"
				$f.ShowNewFolderButton = $true
				if($f.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK){
					$f.SelectedPath
				}
				`
				cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
				out, err := cmd.Output()
				if err != nil || len(out) == 0 {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte(`{"error": "Ingen mapp valdes eller så avbröts dialogen."}`))
					return
				}
				
				selectedWorkspace = strings.TrimSpace(string(out))
				
				if selectedWorkspace == "" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte(`{"error": "Mappen var ogiltig."}`))
					return
				}

				if isRootDirectory(selectedWorkspace) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte(`{"error": "Du kan inte välja en hel rot-enhet (t.ex. C:\\) direkt. Skapa eller välj en specifik undermapp (t.ex. C:\\LocalLedger_Data) för att hålla dina filer rena."}`))
					return
				}
			} else {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error": "Okänd åtgärd"}`))
				return
			}

			// Skicka tillbaka framgång till klienten
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(fmt.Sprintf(`{"redirect": "http://127.0.0.1:%d/"}`, port)))
			
			// Signallera main att vi är klara
			resultChan <- selectedWorkspace
			return
		}

		if r.Method == http.MethodPost && r.URL.Path == "/select-folder" {
			psScript := `
			Add-Type -AssemblyName System.windows.forms
			$f = New-Object System.Windows.Forms.FolderBrowserDialog
			$f.Description = "Välj var du vill återställa din LocalLedger bokföring"
			$f.ShowNewFolderButton = $true
			if($f.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK){
				$f.SelectedPath
			}
			`
			cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
			out, err := cmd.Output()
			if err != nil || len(out) == 0 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error": "Ingen mapp valdes eller så avbröts dialogen."}`))
				return
			}
			
			folderPath := strings.TrimSpace(string(out))
			if folderPath == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error": "Mappen var ogiltig."}`))
				return
			}

			if isRootDirectory(folderPath) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error": "Du kan inte välja en hel rot-enhet (t.ex. C:\\) direkt. Skapa eller välj en specifik undermapp (t.ex. C:\\LocalLedger_Data) för att hålla dina filer rena."}`))
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(fmt.Sprintf(`{"folder": %q}`, folderPath)))
			return
		}

		if r.Method == http.MethodPost && r.URL.Path == "/restore" {
			if err := r.ParseMultipartForm(50 << 20); err != nil { // 50 MB
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error": "Kunde inte tolka formulärdata"}`))
				return
			}

			file, _, err := r.FormFile("backup_zip")
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error": "Ingen fil uppladdad"}`))
				return
			}
			defer file.Close()

			targetWorkspace := r.FormValue("target_workspace")
			if targetWorkspace == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error": "Ingen målmapp angiven"}`))
				return
			}

			if isRootDirectory(targetWorkspace) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error": "Du kan inte välja en hel rot-enhet (t.ex. C:\\) direkt. Skapa eller välj en specifik undermapp (t.ex. C:\\LocalLedger_Data) för att hålla dina filer rena."}`))
				return
			}

			// Skapa målmappen om den inte finns
			if err := os.MkdirAll(targetWorkspace, 0755); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf(`{"error": "Kunde inte skapa målmapp: %v"}`, err)))
				return
			}

			// Spara uppladdad fil temporärt
			tempZip, err := os.CreateTemp("", "setup_upload_*.zip")
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error": "Serverfel vid skapande av temporär fil"}`))
				return
			}
			defer os.Remove(tempZip.Name())

			if _, err := io.Copy(tempZip, file); err != nil {
				tempZip.Close()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error": "Serverfel vid skrivning av fil"}`))
				return
			}
			tempZip.Close()

			// Kontrollera om filen är krypterad
			sigBuf := make([]byte, 8)
			fCheck, err := os.Open(tempZip.Name())
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error": "Serverfel vid öppning av fil"}`))
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
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error": "Serverfel vid läsning av fil"}`))
					return
				}

				decryptedBytes, err := decryptPayload(encryptedBytes, password)
				if err != nil {
					log.Printf("Setup decryption failed: %v", err)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte(`{"error": "invalid_password"}`))
					return
				}

				decryptedZip, err := os.CreateTemp("", "setup_decrypted_*.zip")
				if err != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error": "Serverfel vid skapande av temporär dekrypterad fil"}`))
					return
				}
				defer os.Remove(decryptedZip.Name())

				if _, err := decryptedZip.Write(decryptedBytes); err != nil {
					decryptedZip.Close()
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error": "Serverfel vid skrivning av dekrypterad fil"}`))
					return
				}
				decryptedZip.Close()
				finalZipPath = decryptedZip.Name()
			} else {
				finalZipPath = tempZip.Name()
			}

			// Skapa en "Staging"-mapp inuti targetWorkspace
			stagingDir, err := os.MkdirTemp(targetWorkspace, ".restore_staging_*")
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error": "Serverfel vid skapande av staging-mapp"}`))
				return
			}
			defer os.RemoveAll(stagingDir)

			// Skapa underkataloger i staging
			os.MkdirAll(filepath.Join(stagingDir, "attachments"), 0755)

			// Packa upp säkert (Anti-Zip Slip)
			zipReader, err := zip.OpenReader(finalZipPath)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error": "Ogiltig eller korrupt zip-fil"}`))
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
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(fmt.Sprintf(`{"error": "Kunde inte skapa uppackad fil %s: %v"}`, cleanedName, err)))
					return
				}
				rc, err := f.Open()
				if err != nil {
					outFile.Close()
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(fmt.Sprintf(`{"error": "Kunde inte läsa fil %s ur zip: %v"}`, cleanedName, err)))
					return
				}
				_, errCopy := io.Copy(outFile, rc)
				outFile.Close()
				rc.Close()
				if errCopy != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(fmt.Sprintf(`{"error": "Kunde inte skriva fil %s ur zip: %v"}`, cleanedName, errCopy)))
					return
				}
			}

			if !hasLedgerDB {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error": "Ogiltig säkerhetskopia: ledger.db saknas"}`))
				return
			}

			// Validera den uppackade databasen (v3.0.0)
			tempLedger, err := ledger.OpenLedger(stagingDir, "v3.0.0")
			if err != nil {
				log.Printf("Setup restore validation failed: %v", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(fmt.Sprintf(`{"error": "Säkerhetskopian är ogiltig eller från en inkompatibel version: %v"}`, err)))
				return
			}
			tempLedger.Close()

			// Flytta filer från staging till målmappen
			dbPath := filepath.Join(targetWorkspace, "ledger.db")
			walPath := filepath.Join(targetWorkspace, "ledger.db-wal")
			shmPath := filepath.Join(targetWorkspace, "ledger.db-shm")
			attachPath := filepath.Join(targetWorkspace, "attachments")

			// Rensa eventuella gamla filer i målkatalogen med retry-loopar för Windows
			os.Remove(walPath)
			os.Remove(shmPath)
			
			var removeErr error
			for i := 0; i < 5; i++ {
				removeErr = os.Remove(dbPath)
				if removeErr == nil || os.IsNotExist(removeErr) {
					break
				}
				log.Printf("[Setup Restore] Database remove locked, retrying in 200ms... (%d/5)", i+1)
				time.Sleep(200 * time.Millisecond)
			}
			os.RemoveAll(attachPath)

			// Flytta nya filer på plats med retry-loop
			var renameErr error
			for i := 0; i < 5; i++ {
				renameErr = os.Rename(filepath.Join(stagingDir, "ledger.db"), dbPath)
				if renameErr == nil {
					break
				}
				log.Printf("[Setup Restore] Database rename locked, retrying in 200ms... (%d/5)", i+1)
				time.Sleep(200 * time.Millisecond)
			}
			if renameErr != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf(`{"error": "Kunde inte flytta databasfilen på plats: %v"}`, renameErr)))
				return
			}

			// Flytta attachments om det finns
			if _, errStat := os.Stat(filepath.Join(stagingDir, "attachments")); errStat == nil {
				os.Rename(filepath.Join(stagingDir, "attachments"), attachPath)
			}

			// Flytta eventuella andra filer (som company_logo.* eller annat)
			if files, errRead := os.ReadDir(stagingDir); errRead == nil {
				for _, f := range files {
					if !f.IsDir() && f.Name() != "ledger.db" {
						dest := filepath.Join(targetWorkspace, f.Name())
						os.Remove(dest)
						os.Rename(filepath.Join(stagingDir, f.Name()), dest)
					}
				}
			}

			// Svara med redirect-länk
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(fmt.Sprintf(`{"redirect": "http://127.0.0.1:%d/"}`, port)))

			// Signallera framgång till main thread
			resultChan <- targetWorkspace
			return
		}

		http.Error(w, "Not found", http.StatusNotFound)
	})

	// Starta servern asynkront
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Bakgrundstråd för timeout
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pingMu.Lock()
				if time.Since(lastPing) > 10*time.Second {
					pingMu.Unlock()
					errChan <- fmt.Errorf("Setup-fönstret stängdes (timeout)")
					return
				}
				pingMu.Unlock()
			case <-ctxHeartbeat.Done():
				// Main har returnerat, vi kan stänga ner goroutinen säkert
				return
			}
		}
	}()

	// Öppna webbläsaren
	OpenBrowserAppMode(fmt.Sprintf("http://127.0.0.1:%d", port))

	// Vänta på användarens val (eller uppstartsfel)
	select {
	case err := <-errChan:
		return "", err
	case result := <-resultChan:
		// Stäng ner Setup-servern mjukt så klienten hinner få sin JSON response
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		
		// Skapa skrivbordsgenväg (.lnk) vid slutförd installation
		createDesktopShortcut()

		return result, nil
	}
}

// createDesktopShortcut skapar en Windows-genväg (.lnk) på användarens skrivbord
func createDesktopShortcut() {
	exePath, err := exec.LookPath(os.Args[0])
	if err != nil {
		exePath, err = os.Executable()
		if err != nil {
			return
		}
	}
	
	absExe, err := filepath.Abs(exePath)
	if err != nil {
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	desktopPath := filepath.Join(homeDir, "Desktop", "LocalLedger.lnk")
	workingDir := filepath.Dir(absExe)

	psScript := fmt.Sprintf(`
		$WshShell = New-Object -ComObject WScript.Shell
		$Shortcut = $WshShell.CreateShortcut(%q)
		$Shortcut.TargetPath = %q
		$Shortcut.WorkingDirectory = %q
		$Shortcut.Description = "LocalLedger Bokföring"
		$Shortcut.Save()
	`, desktopPath, absExe, workingDir)

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
	cmd.Run()
}

// isRootDirectory kontrollerar om en sökväg är en rå volym/rot-enhet på Windows
func isRootDirectory(path string) bool {
	if path == "" {
		return false
	}
	cleaned := filepath.Clean(path)
	vol := filepath.VolumeName(cleaned)
	return cleaned == vol || cleaned == vol+string(filepath.Separator)
}
