package ledger

import (
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"path/filepath"
	"time"

	"localledger/internal/models"
)

// GenerateSamlingsplan skapar en HTML-samlingsplan enligt kraven för BFL och BFNAR 2013:2.
func (l *Ledger) GenerateSamlingsplan() (string, error) {
	// Hämta första och senaste verifikations-ID
	var firstID, lastID sql.NullInt64
	err := l.db.QueryRow("SELECT MIN(id), MAX(id) FROM verifications").Scan(&firstID, &lastID)
	if err != nil {
		return "", fmt.Errorf("failed to get verification range: %w", err)
	}

	// Kontrollera WORM-status
	wormValid, wormErr := l.VerifyChain()
	wormStatusText := "Aktiv och Intakt"
	if wormErr != nil {
		wormStatusText = fmt.Sprintf("FEL: %v", wormErr)
	} else if !wormValid {
		wormStatusText = "FEL: Kedjan är ogiltig."
	}

	attachmentsPath := filepath.Join(l.workspacePath, "attachments")
	dbPath := filepath.Join(l.workspacePath, "ledger.db")
	generationTime := time.Now().Format("2006-01-02 15:04:05")

	data := struct {
		GenerationTime string
		AppVersion     string
		Env            string
		WormValid      bool
		WormStatusText string
		DbPath         string
		AttachmentsPath string
		FirstID        int64
		LastID         int64
	}{
		GenerationTime: generationTime,
		AppVersion:     l.appVersion,
		Env:            map[bool]string{true: "Sandbox", false: "Produktion"}[l.isSandbox],
		WormValid:      wormValid,
		WormStatusText: wormStatusText,
		DbPath:         dbPath,
		AttachmentsPath: attachmentsPath,
		FirstID:        firstID.Int64,
		LastID:         lastID.Int64,
	}

	tmplStr := `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Samlingsplan - Systemdokumentation</title>
<style>
  body { font-family: sans-serif; margin: 40px; color: #333; line-height: 1.6; }
  h1 { border-bottom: 2px solid #333; padding-bottom: 10px; }
  table { border-collapse: collapse; width: 100%; margin-top: 20px; }
  th, td { border: 1px solid #ccc; padding: 10px; text-align: left; }
  th { background-color: #f4f4f4; width: 30%; }
  .status-ok { color: green; font-weight: bold; }
  .status-err { color: red; font-weight: bold; }
</style>
</head>
<body>
  <h1>Samlingsplan för Bokföring</h1>
  <p>Genererad: {{.GenerationTime}}</p>
  <p>Detta dokument beskriver systemets struktur och hur räkenskapsinformation lagras och skyddas i enlighet med Bokföringslagen (BFL) kap. 7 och BFNAR 2013:2.</p>
  
  <h2>Systeminformation</h2>
  <table>
    <tr><th>Applikation</th><td>LocalLedger {{.AppVersion}}</td></tr>
    <tr><th>Körläge</th><td>{{.Env}}</td></tr>
    <tr><th>WORM-Skydd (Obrytbar Kedja)</th><td><span class="{{if .WormValid}}status-ok{{else}}status-err{{end}}">{{.WormStatusText}}</span></td></tr>
  </table>

  <h2>Lagring och Sökvägar</h2>
  <table>
    <tr><th>Huvudbok (Databas)</th><td>{{.DbPath}}</td></tr>
    <tr><th>Digitala Original (Kvitton)</th><td>{{.AttachmentsPath}}</td></tr>
    <tr><th>Lagringsformat Kvitton</th><td>Content-Addressable Storage (SHA-256)</td></tr>
  </table>

  <h2>Omfattning</h2>
  <table>
    <tr><th>Första Verifikations-ID</th><td>{{.FirstID}}</td></tr>
    <tr><th>Sista Verifikations-ID</th><td>{{.LastID}}</td></tr>
  </table>
  
  <h2>Rutiner för Säkerhetskopiering</h2>
  <p>Systemet drivs on-premise. Användaren ansvarar för att ta dagliga säkerhetskopior av hela lagringsmappen (både databas och kvitton) till en separat fysisk lagringsmedia (ex. extern hårddisk eller molnlagring).</p>
</body>
</html>`

	tmpl, err := template.New("samlingsplan").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// GetFinancialReport genererar Resultat- och Balansräkning för ett specifikt räkenskapsår.
func (l *Ledger) GetFinancialReport(yearID *int64) (*models.FinancialReport, error) {
	// 1. Hämta räkenskapsår
	var fy models.FiscalYear
	var err error
	
	if yearID != nil {
		err = l.db.QueryRow("SELECT id, start_date, end_date FROM fiscal_years WHERE id = ?", *yearID).Scan(&fy.ID, &fy.StartDate, &fy.EndDate)
	} else {
		err = l.db.QueryRow("SELECT id, start_date, end_date FROM fiscal_years ORDER BY id DESC LIMIT 1").Scan(&fy.ID, &fy.StartDate, &fy.EndDate)
	}
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("Inget räkenskapsår hittades")
		}
		return nil, fmt.Errorf("failed to get fiscal year: %w", err)
	}

	report := &models.FinancialReport{
		FiscalYear:  fmt.Sprintf("%s — %s", fy.StartDate, fy.EndDate),
		Income:      []models.ReportRow{},
		Expenses:    []models.ReportRow{},
		Assets:      []models.ReportRow{},
		Liabilities: []models.ReportRow{},
	}

	// 2. Hämta aggregerade summor för alla använda konton under räkenskapsåret
	query := `
		SELECT 
			a.code, 
			a.name, 
			a.type,
			SUM(r.debet) as total_debet,
			SUM(r.kredit) as total_kredit
		FROM verification_rows r
		JOIN verifications v ON r.verification_id = v.id
		JOIN accounts a ON r.account = a.code
		WHERE v.date >= ? AND v.date <= ?
		GROUP BY a.code, a.name, a.type
		HAVING total_debet > 0 OR total_kredit > 0
		ORDER BY a.code ASC
	`

	rows, err := l.db.Query(query, fy.StartDate, fy.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query report data: %w", err)
	}
	defer rows.Close()

	var totalIncome int64
	var totalExpenses int64
	var totalAssets int64
	var totalLiabilities int64

	for rows.Next() {
		var code, name, accType string
		var debet, kredit int64
		if err := rows.Scan(&code, &name, &accType, &debet, &kredit); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Tilldela rätt teckenpolaritet beroende på kontotyp (för UX)
		switch accType {
		case "Intäkt":
			balance := kredit - debet
			if balance != 0 {
				report.Income = append(report.Income, models.ReportRow{AccountCode: code, AccountName: name, Balance: balance})
				totalIncome += balance
			}
		case "Kostnad":
			balance := debet - kredit
			if balance != 0 {
				report.Expenses = append(report.Expenses, models.ReportRow{AccountCode: code, AccountName: name, Balance: balance})
				totalExpenses += balance
			}
		case "Tillgång":
			balance := debet - kredit
			if balance != 0 {
				report.Assets = append(report.Assets, models.ReportRow{AccountCode: code, AccountName: name, Balance: balance})
				totalAssets += balance
			}
		case "Skuld":
			balance := kredit - debet
			if balance != 0 {
				report.Liabilities = append(report.Liabilities, models.ReportRow{AccountCode: code, AccountName: name, Balance: balance})
				totalLiabilities += balance
			}
		}
	}

	report.TotalIncome = totalIncome
	report.TotalExpenses = totalExpenses
	report.NetIncome = totalIncome - totalExpenses
	
	report.TotalAssets = totalAssets
	report.TotalLiabilities = totalLiabilities
	report.CalculatedEquity = totalLiabilities + report.NetIncome

	return report, nil
}
