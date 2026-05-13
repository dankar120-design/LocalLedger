package inbox

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"localledger/internal/models"
)

type InboxManager struct {
	db            *sql.DB
	workspacePath string
	inboxPath     string
}

func NewInboxManager(db *sql.DB, workspacePath string) *InboxManager {
	inboxPath := filepath.Join(workspacePath, "inbox")
	if err := os.MkdirAll(inboxPath, 0755); err != nil {
		log.Printf("Failed to create inbox directory: %v", err)
	}

	manager := &InboxManager{
		db:            db,
		workspacePath: workspacePath,
		inboxPath:     inboxPath,
	}

	// Kör orphan reconciliation vid uppstart synkront
	manager.ReconcileOrphans()

	return manager
}

func (m *InboxManager) ValidateFile(data []byte, originalFilename string) (string, error) {
	// 1. Storleksgräns (Max 20MB)
	if len(data) > 20*1024*1024 {
		return "", errors.New("file exceeds 20MB limit")
	}

	// 2. Filändelse-whitelist
	ext := strings.ToLower(filepath.Ext(originalFilename))
	allowedExts := map[string]bool{
		".pdf":  true,
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".webp": true,
	}
	if !allowedExts[ext] {
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}

	// 3. MIME-typ kontroll (Magic Bytes)
	// http.DetectContentType behöver bara de första 512 byten
	mimeType := http.DetectContentType(data)

	// Strikt korrelation mellan ändelse och MIME
	valid := false
	switch ext {
	case ".pdf":
		valid = mimeType == "application/pdf"
	case ".png", ".jpg", ".jpeg", ".webp":
		valid = mimeType == "image/png" || mimeType == "image/jpeg" || mimeType == "image/webp"
	}

	if !valid {
		return "", fmt.Errorf("security violation: file extension %s does not match detected MIME type %s", ext, mimeType)
	}

	return mimeType, nil
}

func (m *InboxManager) SaveFile(data []byte, originalFilename string, source string) (*models.InboxItem, error) {
	mimeType, err := m.ValidateFile(data, originalFilename)
	if err != nil {
		return nil, err
	}

	// Generera UUID-liknande ID
	b := make([]byte, 16)
	rand.Read(b)
	id := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])

	// Generera stored_filename
	ext := filepath.Ext(originalFilename)
	storedFilename := fmt.Sprintf("%s%s", id, ext)
	filePath := filepath.Join(m.inboxPath, storedFilename)

	// Skriv filen till disk
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write file to inbox: %w", err)
	}

	// Skapa DB-rad
	item := &models.InboxItem{
		ID:               id,
		OriginalFilename: originalFilename,
		StoredFilename:   storedFilename,
		FileSize:         int64(len(data)),
		MimeType:         mimeType,
		UploadedAt:       time.Now(),
		Source:           source,
	}

	_, err = m.db.Exec(`
		INSERT INTO inbox_items (id, original_filename, stored_filename, file_size, mime_type, uploaded_at, source)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.OriginalFilename, item.StoredFilename, item.FileSize, item.MimeType, item.UploadedAt, item.Source)

	if err != nil {
		// Rollback filen om DB misslyckas
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to save inbox item to db: %w", err)
	}

	return item, nil
}

func (m *InboxManager) DeleteItem(id string) error {
	var storedFilename string
	err := m.db.QueryRow("SELECT stored_filename FROM inbox_items WHERE id = ?", id).Scan(&storedFilename)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("item not found")
		}
		return err
	}

	// Transaktionellt: Ta bort fil först
	filePath := filepath.Join(m.inboxPath, storedFilename)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Radera från DB
	_, err = m.db.Exec("DELETE FROM inbox_items WHERE id = ?", id)
	return err
}

func (m *InboxManager) GetItemInfo(id string) (mimeType string, filePath string, err error) {
	var storedFilename string
	err = m.db.QueryRow("SELECT mime_type, stored_filename FROM inbox_items WHERE id = ?", id).Scan(&mimeType, &storedFilename)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", errors.New("item not found")
		}
		return "", "", err
	}
	
	filePath = filepath.Join(m.inboxPath, storedFilename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", "", errors.New("file not found on disk")
	}
	
	return mimeType, filePath, nil
}

func (m *InboxManager) GetAllItems() ([]models.InboxItem, error) {
	rows, err := m.db.Query("SELECT id, original_filename, stored_filename, file_size, mime_type, uploaded_at, source FROM inbox_items ORDER BY uploaded_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.InboxItem
	for rows.Next() {
		var i models.InboxItem
		// Parse dates properly based on how SQLite returns them, usually as string or time.Time if parsed correctly via driver
		if err := rows.Scan(&i.ID, &i.OriginalFilename, &i.StoredFilename, &i.FileSize, &i.MimeType, &i.UploadedAt, &i.Source); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, nil
}

func (m *InboxManager) ReconcileOrphans() {
	log.Println("[Inbox] Running Orphan Reconciliation...")
	
	// Läs alla filer i inbox-mappen
	entries, err := os.ReadDir(m.inboxPath)
	if err != nil {
		log.Printf("[Inbox] Reconciliation failed to read dir: %v", err)
		return
	}

	// Hämta alla DB rader
	dbItems, err := m.GetAllItems()
	if err != nil {
		log.Printf("[Inbox] Reconciliation failed to fetch DB: %v", err)
		return
	}

	// Skapa map av DB filer
	dbFiles := make(map[string]bool)
	for _, item := range dbItems {
		dbFiles[item.StoredFilename] = true
	}

	orphans := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		if !dbFiles[filename] {
			log.Printf("[Inbox] WARN: Orphaned file found on disk: %s. Leaving it for manual inspection.", filename)
			orphans++
		}
	}

	if orphans == 0 {
		log.Println("[Inbox] Reconciliation complete. No orphans found.")
	} else {
		log.Printf("[Inbox] Reconciliation complete. %d orphans found.", orphans)
	}
}
