package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// handleUploadLogo hanterar uppladdning av företagslogotyp (SVG, PNG, JPG/JPEG, max 5MB)
func (s *Server) handleUploadLogo(w http.ResponseWriter, r *http.Request) {
	// Begränsa storlek till 5MB
	r.Body = http.MaxBytesReader(w, r.Body, 5*1024*1024)
	if err := r.ParseMultipartForm(5 * 1024 * 1024); err != nil {
		writeError(w, fmt.Errorf("filen är för stor (max 5MB): %w", err))
		return
	}

	file, header, err := r.FormFile("logo_file")
	if err != nil {
		writeError(w, fmt.Errorf("logo_file fält saknas: %w", err))
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".svg" && ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		writeError(w, errors.New("endast SVG, PNG och JPG/JPEG logotyper är tillåtna"))
		return
	}

	// 1. Läs första 512 bytes för MIME magic bytes kontroll
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		writeError(w, fmt.Errorf("kunde inte läsa filens magiska byte: %w", err))
		return
	}
	// Återställ filpekare
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, fmt.Errorf("kunde inte återställa filpekare: %w", err))
		return
	}

	mimeType := http.DetectContentType(buf[:n])

	// Validera MIME-typ mot förväntad filändelse för att förhindra masquerading
	if ext == ".svg" {
		if !strings.Contains(mimeType, "xml") && !strings.Contains(mimeType, "plain") && !strings.Contains(mimeType, "svg") {
			writeError(w, errors.New("uppladdad SVG-fil har ogiltig MIME-typ"))
			return
		}
	} else if ext == ".png" {
		if mimeType != "image/png" {
			writeError(w, errors.New("Kunde inte ladda upp logotyp: filen har en ogiltig intern MIME-typ. Detta händer oftast om en JPG-, WebP- eller PDF-fil har döpts om till .png manuellt. Spara om filen i ett riktigt bildprogram."))
			return
		}
	} else if ext == ".jpg" || ext == ".jpeg" {
		if mimeType != "image/jpeg" {
			writeError(w, errors.New("uppladdad JPG/JPEG-fil har ogiltig MIME-typ (matchar inte image/jpeg)"))
			return
		}
	}

	// 2. Om filen är en SVG, läs hela innehållet för att skanna efter XSS-kod (Stored XSS-skydd)
	if ext == ".svg" {
		svgContent, err := io.ReadAll(file)
		if err != nil {
			writeError(w, fmt.Errorf("kunde inte läsa SVG-innehåll: %w", err))
			return
		}
		// Återställ filpekare igen inför disk-skrivning
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			writeError(w, fmt.Errorf("kunde inte återställa filpekare efter SVG-läsning: %w", err))
			return
		}

		if isSVGXSS(svgContent) {
			writeError(w, errors.New("säkerhetsfel: SVG-filen innehåller otillåtna skript eller event-lyssnare (Stored XSS-skydd)"))
			return
		}
	}

	// Hämta befintliga inställningar för att kunna ta bort eventuell gammal logga
	settings, err := s.ledger.GetSettings()
	if err != nil {
		writeError(w, err)
		return
	}

	workspace := s.ledger.WorkspacePath()
	logoFilename := "company_logo" + ext
	newLogoPath := filepath.Join(workspace, logoFilename)

	// Om gammal logga finns och är en annan fil, ta bort den för att hålla rent
	if settings.LogoPath != "" && settings.LogoPath != logoFilename {
		oldPath := filepath.Join(workspace, settings.LogoPath)
		_ = os.Remove(oldPath) // Ignorera om filen redan är borta
	}

	// Skapa/skriv den nya filen
	out, err := os.Create(newLogoPath)
	if err != nil {
		writeError(w, fmt.Errorf("kunde inte spara logotypen på disk: %w", err))
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		writeError(w, fmt.Errorf("fel vid skrivning av logotyp: %w", err))
		return
	}

	// Uppdatera databasinställningar med filnamnet
	settings.LogoPath = logoFilename
	if err := s.ledger.UpdateSettings(settings); err != nil {
		writeError(w, fmt.Errorf("kunde inte spara logotypsökväg i inställningar: %w", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(settings)
}

// handleServeLogo läser logotypen från disk och serverar den
func (s *Server) handleServeLogo(w http.ResponseWriter, r *http.Request) {
	settings, err := s.ledger.GetSettings()
	if err != nil {
		writeError(w, err)
		return
	}

	if settings.LogoPath == "" {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Ingen logotyp uppladdad"))
		return
	}

	logoPath := filepath.Join(s.ledger.WorkspacePath(), settings.LogoPath)

	// Kolla om filen finns fysiskt
	if _, err := os.Stat(logoPath); os.IsNotExist(err) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Logotypfilen hittades inte på disk"))
		return
	}

	// Sätt Content-Type baserat på filändelse
	ext := strings.ToLower(filepath.Ext(settings.LogoPath))
	switch ext {
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	// Strikta säkerhetshuvuden för filservering
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox;")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	http.ServeFile(w, r, logoPath)
}

// isSVGXSS scannar en SVG efter Stored XSS-vektorer (skript och inline-eventlyssnare)
func isSVGXSS(data []byte) bool {
	content := strings.ToLower(string(data))

	// Hitta script-element
	if strings.Contains(content, "<script") || strings.Contains(content, "xmlns:script") {
		return true
	}

	// Hitta javascript: URI-schema
	if strings.Contains(content, "javascript:") {
		return true
	}

	// Hitta inline event-handlers (t.ex. onload= eller onload =)
	eventHandlers := []string{
		"onload", "onclick", "onmouseover", "onfocus", "onerror",
		"onunload", "onchange", "onsubmit", "onreset", "onselect",
		"onblur", "onkeydown", "onkeypress", "onkeyup",
	}
	for _, eh := range eventHandlers {
		if strings.Contains(content, eh) {
			idx := strings.Index(content, eh)
			for idx != -1 {
				sub := content[idx+len(eh):]
				subTrimmed := strings.TrimSpace(sub)
				if len(subTrimmed) > 0 && subTrimmed[0] == '=' {
					return true
				}
				next := strings.Index(sub, eh)
				if next == -1 {
					break
				}
				idx += len(eh) + next
			}
		}
	}

	return false
}
