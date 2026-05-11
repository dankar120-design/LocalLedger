package models

type Account struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type VerificationRequest struct {
	Date             string
	Text             string
	Type             string // 'NORMAL', 'IB', 'MAKULERAD' etc.
	AttachmentBase64 string
	Rows             []RowRequest
}

type RowRequest struct {
	Account string
	Debet   int64
	Kredit  int64
}

type VerificationResult struct {
	ID        int64
	CreatedAt string
}

type VerificationResponse struct {
	ID             int64         `json:"id"`
	CreatedAt      string        `json:"created_at"`
	Date           string        `json:"date"`
	Text           string        `json:"text"`
	Type           string        `json:"type"`
	Hash           *string       `json:"hash,omitempty"`
	IsStornoed     bool          `json:"is_stornoed"`
	StornoRefID    *int64        `json:"storno_ref_id,omitempty"`
	AttachmentHash *string       `json:"attachment_hash,omitempty"`
	AttachmentMime *string       `json:"attachment_mime,omitempty"`
	Rows           []RowResponse `json:"rows"`
}

type RowResponse struct {
	ID      int64  `json:"id"`
	Account string `json:"account"`
	Debet   int64  `json:"debet"`
	Kredit  int64  `json:"kredit"`
}

type FiscalYear struct {
	ID        int64  `json:"id"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	IsLocked  bool   `json:"is_locked"`
}

type SealResult struct {
	Count    int
	LastHash string
	FirstID  int64
	LastID   int64
}

type ReportRow struct {
	AccountCode string
	AccountName string
	Balance     int64 // I ören. Tecken hanteras i presentationslagret/backend
}

type FinancialReport struct {
	FiscalYear   string
	Income       []ReportRow
	Expenses     []ReportRow
	Assets       []ReportRow
	Liabilities  []ReportRow
	NetIncome    int64
	TotalIncome  int64
	TotalExpenses int64
	TotalAssets  int64
	TotalLiabilities int64
	CalculatedEquity int64
}

type CompanySettings struct {
	Name      string `json:"name"`
	OrgNumber string `json:"org_number"`
}
