package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"localledger/frontend"
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
		return result, nil
	}
}
