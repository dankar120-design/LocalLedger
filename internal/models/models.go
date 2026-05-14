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
	Name               string `json:"name"`
	OrgNumber          string `json:"org_number"`
	CloudInboxPath     string `json:"cloud_inbox_path"`
	Address            string `json:"address"`
	Bankgiro           string `json:"bankgiro"`
	SwishNumber        string `json:"swish_number"`
	InvoiceStartNumber int    `json:"invoice_start_number"`
	PaymentTermsDays   int    `json:"payment_terms_days"`
	LogoPath           string `json:"logo_path"`
}

type Invoice struct {
	ID               int64         `json:"id"`
	InvoiceNumber    *string       `json:"invoice_number,omitempty"`
	Date             string        `json:"date"`
	DueDate          string        `json:"due_date"`
	PaymentTermsDays int           `json:"payment_terms_days"`
	CustomerName     string        `json:"customer_name"`
	CustomerOrgnr    string        `json:"customer_orgnr"`
	CustomerAddress  string        `json:"customer_address"`
	TotalAmount      int64         `json:"total_amount"` // i ören (10000 = 100.00 kr)
	TotalVat         int64         `json:"total_vat"`    // i ören
	Status           string        `json:"status"`
	VerificationID   *int64        `json:"verification_id,omitempty"`
	CreditOf         *int64        `json:"credit_of,omitempty"`
	FiscalYearID     int64         `json:"fiscal_year_id"`
	CreatedAt        string        `json:"created_at"`
	Items            []InvoiceItem `json:"items"`
}

type InvoiceItem struct {
	ID          int64  `json:"id"`
	InvoiceID   int64  `json:"invoice_id"`
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`     // i hundradelar (150 = 1.5 enheter)
	PriceExVat  int64  `json:"price_ex_vat"` // i ören (10000 = 100.00 kr)
	VatRate     int    `json:"vat_rate"`
}

type DashboardMetrics struct {
	BankBalance int64 `json:"bank_balance"` // Summan av alla konton som börjar på 19 (tillgångar)
	NetIncome   int64 `json:"net_income"`   // Årets resultat (Intäkter - Kostnader)
	Income      int64 `json:"income"`       // Endast intäkter för presentation
	Expenses    int64 `json:"expenses"`     // Endast kostnader för presentation
}
