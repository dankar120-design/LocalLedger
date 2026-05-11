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
	"localledger/internal/ledger"
)

// Server kapslar in HTTP-servern och bokföringsmotorn.
type Server struct {
	httpServer   *http.Server
	ledger       *ledger.Ledger
	token        string
	indexTmpl    *template.Template
	shutdownChan chan struct{}
	shutdownOnce sync.Once
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
func Start(workspace string, port int) (*Server, error) {
	// 0. Bind till porten synkront för att fånga fel direkt (t.ex. om porten redan används)
	// Detta agerar som en applikations-mutex för att garantera single-instance.
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind to %s (is the port already in use?): %w", addr, err)
	}

	// 1. Öppna LocalLedger
	l, err := ledger.OpenLedger(workspace, "v1.4.0")
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
		ledger:       l,
		token:        token,
		indexTmpl:    indexTmpl,
		shutdownChan: make(chan struct{}),
	}

	// 3. Sätt upp routes (Go 1.22+ ServeMux)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/shutdown", s.handleShutdown)
	s.registerRoutes(mux)

	// Frontend routes
	staticFS, err := fs.Sub(frontend.FS, "static")
	if err != nil {
		listener.Close()
		return nil, fmt.Errorf("failed to sub embedded static dir: %w", err)
	}
	mux.HandleFunc("GET /", s.handleFrontendIndex)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Lägg på Authentication Middleware
	handler := s.authMiddleware(mux)

	// 4. Konfigurera HTTP-servern
	s.httpServer = &http.Server{
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// 6. Starta servern i bakgrunden
	go func() {
		log.Printf("Starting LocalLedger API on http://%s", addr)
		log.Printf("SECURITY TOKEN: %s", token)
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
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

// authMiddleware kräver 'Authorization: Bearer <token>' för alla /api/-anrop.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Vi vill skydda allt under /api/ utom /api/attachments/, /api/reports/ och /api/export/
		if strings.HasPrefix(r.URL.Path, "/api/") && !strings.HasPrefix(r.URL.Path, "/api/attachments/") && !strings.HasPrefix(r.URL.Path, "/api/reports/") && !strings.HasPrefix(r.URL.Path, "/api/export/") {
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
		Token string
	}{
		Token: s.token,
	}

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
		"version": "v1.4.0",
	}

	if err == nil && fy != nil {
		status["fiscal_year"] = fy
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}


