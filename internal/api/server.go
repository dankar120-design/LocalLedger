package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"localledger/frontend"
	"localledger/internal/inbox"
	"localledger/internal/ledger"
)

const CurrentAppVersion = "1.4.0"

type Server struct {
	httpServer   *http.Server
	listener     net.Listener
	ledger       *ledger.Ledger
	inbox        *inbox.InboxManager
	token        string
	indexTmpl    *template.Template
	isSandbox    bool
	shutdownChan chan struct{}
	shutdownOnce sync.Once
	
	// Heartbeat watchdog
	lastPing     time.Time
	pingMu       sync.Mutex
}

// GenerateToken skapar en slumpmässig 32-byte hex sträng.
func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random token: %v", err))
	}
	return hex.EncodeToString(b)
}

// Start initierar och startar servern. Returnerar server-instansen.
// Den blockerar INTE, utan startar servern i en goroutine.
func Start(workspace string, port int, isE2E bool, isSandbox bool) (*Server, error) {
	// 0. Bind till porten synkront för att fånga fel direkt (t.ex. om porten redan används)
	// Detta agerar som en applikations-mutex för att garantera single-instance.
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind to %s (is the port already in use?): %w", addr, err)
	}

	// 1. Öppna LocalLedger
	l, err := ledger.OpenLedger(workspace, "v3.0.0")
	if err != nil {
		listener.Close() // Stäng listener om vi misslyckas efter bind
		return nil, fmt.Errorf("failed to open ledger: %w", err)
	}

	// 2. Generera säkerhetstoken
	token := generateToken()

	// Läs in index.html från embed.FS och parsa den
	indexBytes, err := frontend.FS.ReadFile("views/index.html")
	if err != nil {
		listener.Close()
		return nil, fmt.Errorf("failed to read embedded index.html: %w", err)
	}
	indexTmpl, err := template.New("index").Parse(string(indexBytes))
	if err != nil {
		listener.Close()
		return nil, fmt.Errorf("failed to parse index.html template: %w", err)
	}

	s := &Server{
		listener:     listener,
		ledger:       l,
		inbox:        inbox.NewInboxManager(l.DB(), workspace),
		token:        token,
		indexTmpl:    indexTmpl,
		isSandbox:    isSandbox,
		shutdownChan: make(chan struct{}),
		lastPing:     time.Now(),
	}

	// 3. Sätt upp routes (Go 1.22+ ServeMux)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/shutdown", s.handleShutdown)
	mux.HandleFunc("GET /api/ping", s.handlePing)
	mux.HandleFunc("POST /api/ping", s.handlePing)
	s.registerRoutes(mux)

	// Frontend routes
	staticFS, err := fs.Sub(frontend.FS, "static")
	if err != nil {
		listener.Close()
		return nil, fmt.Errorf("failed to sub embedded static dir: %w", err)
	}
	mux.HandleFunc("GET /", s.handleFrontendIndex)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		http.FileServer(http.FS(staticFS)).ServeHTTP(w, r)
	})))

	// Lägg på Authentication Middleware
	handler := s.authMiddleware(mux)

	// 4. Konfigurera HTTP-servern
	s.httpServer = &http.Server{
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// 6. Starta servern i bakgrunden
	go func() {
		log.Printf("Starting LocalLedger API on http://%s", addr)
		log.Printf("SECURITY TOKEN: %s", token)
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// 7. Starta heartbeat watchdog för att förhindra zombie-processer
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.pingMu.Lock()
				idleTime := time.Since(s.lastPing)
				s.pingMu.Unlock()
				
				if idleTime > 90*time.Second {
					log.Printf("[Watchdog] No heartbeat received for %v. Initiating auto-shutdown...", idleTime)
					s.shutdownOnce.Do(func() {
						close(s.shutdownChan)
					})
					return
				}
			case <-s.shutdownChan:
				return
			}
		}
	}()

	return s, nil
}

// Shutdown stänger ner HTTP-servern och databasen säkert.
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down API server...")
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("HTTP shutdown failed: %w", err)
	}

	log.Println("Closing ledger database...")
	if err := s.ledger.Close(); err != nil {
		return fmt.Errorf("ledger close failed: %w", err)
	}

	return nil
}

// ShutdownChan returnerar kanalen som stängs när servern ska avslutas.
func (s *Server) ShutdownChan() <-chan struct{} {
	return s.shutdownChan
}

// Token returnerar den genererade säkerhetstokenen för denna session.
func (s *Server) Token() string {
	return s.token
}

// CompanyName returnerar företagsnamnet inställt i databasen.
func (s *Server) CompanyName() string {
	if s.ledger == nil {
		return ""
	}
	settings, err := s.ledger.GetSettings()
	if err != nil {
		return ""
	}
	return settings.Name
}

// IsSandbox returnerar om servern körs i sandbox-läge.
func (s *Server) IsSandbox() bool {
	return s.isSandbox
}


// authMiddleware kräver 'Authorization: Bearer <token>' för alla /api/-anrop.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Vi vill skydda allt under /api/ utom /api/attachments/, /api/reports/samlingsplan, /api/reports/excel och logotypen
		if strings.HasPrefix(r.URL.Path, "/api/") && 
			!strings.HasPrefix(r.URL.Path, "/api/attachments/") && 
			!strings.HasPrefix(r.URL.Path, "/api/reports/samlingsplan") && 
			!strings.HasPrefix(r.URL.Path, "/api/reports/excel") &&
			r.URL.Path != "/api/ping" &&
			!(r.Method == "GET" && r.URL.Path == "/api/settings/logo") {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, "Unauthorized: Missing Bearer Token", http.StatusUnauthorized)
				return
			}

			providedToken := strings.TrimPrefix(authHeader, "Bearer ")
			if providedToken != s.token {
				http.Error(w, "Unauthorized: Invalid Token", http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// handleShutdown stänger shutdown-kanalen så att programmet kan avslutas.
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"shutting down"}`))
	
	// Använd en goroutine för att stänga kanalen så att vi hinner skicka HTTP-svaret
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.shutdownOnce.Do(func() {
			close(s.shutdownChan)
		})
	}()
}

// handleFrontendIndex serverar index.html och injicerar API-token
func (s *Server) handleFrontendIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data := struct {
		Token     string
		IsSandbox bool
	}{
		Token:     s.token,
		IsSandbox: s.isSandbox,
	}

	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.indexTmpl.Execute(w, data); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}

// handleHealth returnerar en enkel status-JSON.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	fy, err := s.ledger.GetActiveFiscalYear()
	
	status := map[string]interface{}{
		"status":  "ok",
		"version": CurrentAppVersion,
	}

	if err == nil && fy != nil {
		status["fiscal_year"] = fy
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// handlePing hanterar keep-alive pings från frontenden.
func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	s.pingMu.Lock()
	s.lastPing = time.Now()
	s.pingMu.Unlock()
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"pong"}`))
}


