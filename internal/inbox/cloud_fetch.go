package inbox

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

type FetchResult struct {
	Fetched int      `json:"fetched"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors"`
}

// FetchFromCloud skannar moln-inkorgen och flyttar in max 20 filer.
func (m *InboxManager) FetchFromCloud() (FetchResult, error) {
	result := FetchResult{Errors: []string{}}
	// 1. Hämta inställningen
	var cloudPath string
	err := m.db.QueryRow("SELECT cloud_inbox_path FROM company_settings WHERE id = 1").Scan(&cloudPath)
	if err != nil {
		if err == sql.ErrNoRows {
			return result, fmt.Errorf("company settings not found")
		}
		return result, err
	}

	if cloudPath == "" {
		return result, fmt.Errorf("cloud inbox path is not configured")
	}

	if _, err := os.Stat(cloudPath); os.IsNotExist(err) {
		return result, fmt.Errorf("cloud inbox path does not exist: %s", cloudPath)
	}

	// Skapa _Processed-mappen
	processedPath := filepath.Join(cloudPath, "_Processed")
	if err := os.MkdirAll(processedPath, 0755); err != nil {
		return result, fmt.Errorf("failed to create _Processed folder in cloud: %w", err)
	}

	// Läs max 20 filer
	entries, err := os.ReadDir(cloudPath)
	if err != nil {
		return result, fmt.Errorf("failed to read cloud directory: %w", err)
	}

	fetchedCount := 0
	failedCount := 0
	for _, entry := range entries {
		if fetchedCount >= 20 {
			log.Println("[Inbox] Batch limit of 20 reached. Stopping cloud fetch early.")
			break
		}

		if entry.IsDir() || entry.Name() == "_Processed" {
			continue
		}

		// 2. File Locking Check (500ms verify)
		sourceFilePath := filepath.Join(cloudPath, entry.Name())
		info1, err := os.Stat(sourceFilePath)
		if err != nil {
			log.Printf("[Inbox] Skipping %s: failed to stat: %v", entry.Name(), err)
			continue
		}

		time.Sleep(500 * time.Millisecond)

		info2, err := os.Stat(sourceFilePath)
		if err != nil {
			log.Printf("[Inbox] Skipping %s: failed to stat on second check: %v", entry.Name(), err)
			continue
		}

		if info1.Size() != info2.Size() {
			log.Printf("[Inbox] Skipping %s: file is still being downloaded by cloud client", entry.Name())
			continue
		}

		if info1.Size() == 0 {
			log.Printf("[Inbox] Skipping %s: file is empty", entry.Name())
			continue
		}

		// 3. Flytta originalet till _Processed FÖRST (säkert för os.Rename inom samma mount)
		// Lägg till timestamp i filnamnet för att förhindra överskrivning av befintliga filer i _Processed
		destFilename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), entry.Name())
		destProcessedPath := filepath.Join(processedPath, destFilename)
		
		if err := os.Rename(sourceFilePath, destProcessedPath); err != nil {
			log.Printf("[Inbox] Failed to move %s to _Processed: %v", entry.Name(), err)
			result.Errors = append(result.Errors, fmt.Sprintf("%s: failed to move (%v)", entry.Name(), err))
			failedCount++
			continue
		}

		// 4. EXDEV-säker ingestering (Copy to C:)
		data, err := os.ReadFile(destProcessedPath)
		if err != nil {
			log.Printf("[Inbox] Failed to read moved file %s: %v", destProcessedPath, err)
			result.Errors = append(result.Errors, fmt.Sprintf("%s: failed to read (%v)", entry.Name(), err))
			failedCount++
			continue
		}

		// Spara till lokal inkorg via InboxManager
		_, err = m.SaveFile(data, entry.Name(), "cloud")
		if err != nil {
			log.Printf("[Inbox] Failed to ingest %s: %v", entry.Name(), err)
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", entry.Name(), err))
			failedCount++
			continue
		}

		fetchedCount++
	}

	result.Fetched = fetchedCount
	result.Failed = failedCount
	return result, nil
}
