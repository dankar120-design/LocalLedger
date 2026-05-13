package models

import "time"

type InboxItem struct {
	ID               string    `json:"id"`
	OriginalFilename string    `json:"original_filename"`
	StoredFilename   string    `json:"stored_filename"`
	FileSize         int64     `json:"file_size"`
	MimeType         string    `json:"mime_type"`
	UploadedAt       time.Time `json:"uploaded_at"`
	Source           string    `json:"source"`
}
