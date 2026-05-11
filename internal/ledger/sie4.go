package ledger

import (
	"bytes"
	"fmt"
	"localledger/internal/models"
	"strings"
	"time"

	"golang.org/x/text/encoding/charmap"
)

func (l *Ledger) GenerateSIE4(yearID int64) ([]byte, error) {
	// 1. Get Fiscal Year
	var fy models.FiscalYear
	err := l.db.QueryRow("SELECT id, start_date, end_date FROM fiscal_years WHERE id = ?", yearID).Scan(&fy.ID, &fy.StartDate, &fy.EndDate)
	if err != nil {
		return nil, fmt.Errorf("fiscal year not found: %w", err)
	}

	// 2. Get Settings
	settings, err := l.GetSettings()
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	// Hjälpfunktion för att ta bort citationstecken som kraschar SIE-formatet
	sanitize := func(s string) string {
		return strings.ReplaceAll(s, "\"", "'")
	}

	// --- Header ---
	buf.WriteString("#FLAGGA 0\r\n")
	buf.WriteString("#FORMAT PC8\r\n")
	buf.WriteString("#SIETYP 4\r\n")
	buf.WriteString("#PROGRAM \"LocalLedger\" 1.4.0\r\n")
	buf.WriteString(fmt.Sprintf("#GEN %s\r\n", time.Now().Format("20060102")))
	buf.WriteString(fmt.Sprintf("#FNAMN \"%s\"\r\n", sanitize(settings.Name)))
	buf.WriteString(fmt.Sprintf("#ORGNR %s\r\n", settings.OrgNumber))
	buf.WriteString("#KPTYP BAS2024\r\n")
	
	startDateClean := strings.ReplaceAll(fy.StartDate[:10], "-", "")
	endDateClean := strings.ReplaceAll(fy.EndDate[:10], "-", "")
	buf.WriteString(fmt.Sprintf("#RAR 0 %s %s\r\n", startDateClean, endDateClean))

	// 3. Accounts & Balances
	rows, err := l.db.Query(`
		SELECT a.code, a.name, 
		       COALESCE((
		           SELECT SUM(vr.debet - vr.kredit) 
		           FROM verification_rows vr 
		           JOIN verifications v ON vr.verification_id = v.id 
		           WHERE vr.account = a.code AND v.date >= ? AND v.date <= ?
		       ), 0) as balance,
		       COALESCE((
		           SELECT SUM(vr.debet - vr.kredit) 
		           FROM verification_rows vr 
		           JOIN verifications v ON vr.verification_id = v.id 
		           WHERE vr.account = a.code AND v.date >= ? AND v.date <= ? AND v.type = 'IB'
		       ), 0) as ib_balance
		FROM accounts a
		ORDER BY a.code
	`, fy.StartDate, fy.EndDate, fy.StartDate, fy.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate balances: %w", err)
	}
	defer rows.Close()

	type AccBalance struct {
		Code      string
		Balance   int64 // i ören, raw Debet - Kredit
		IBBalance int64
	}
	var accBalances []AccBalance

	for rows.Next() {
		var code, name string
		var balance, ibBalance int64
		if err := rows.Scan(&code, &name, &balance, &ibBalance); err != nil {
			return nil, err
		}
		
		// KONTO tag
		buf.WriteString(fmt.Sprintf("#KONTO %s \"%s\"\r\n", code, sanitize(name)))
		accBalances = append(accBalances, AccBalance{Code: code, Balance: balance, IBBalance: ibBalance})
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("account iteration error: %w", err)
	}

	// Output IB, UB, RES
	for _, ab := range accBalances {
		// Konvertera ören till decimal (10050 -> 100.50)
		balanceFloat := float64(ab.Balance) / 100.0
		ibFloat := float64(ab.IBBalance) / 100.0

		// Balanskonton: 1xxx och 2xxx
		if strings.HasPrefix(ab.Code, "1") || strings.HasPrefix(ab.Code, "2") {
			if ab.IBBalance != 0 {
				buf.WriteString(fmt.Sprintf("#IB 0 %s %.2f\r\n", ab.Code, ibFloat))
			} else {
				buf.WriteString(fmt.Sprintf("#IB 0 %s 0.00\r\n", ab.Code))
			}
			if ab.Balance != 0 {
				buf.WriteString(fmt.Sprintf("#UB 0 %s %.2f\r\n", ab.Code, balanceFloat))
			}
		} else {
			// Resultatkonton: 3xxx till 8xxx
			if ab.Balance != 0 {
				buf.WriteString(fmt.Sprintf("#RES 0 %s %.2f\r\n", ab.Code, balanceFloat))
			}
		}
	}

	// 4. Verifications (Exkludera type = 'IB' eftersom de är redovisade ovan)
	vRows, err := l.db.Query(`
		SELECT v.id, v.date, v.text, vr.account, vr.debet, vr.kredit
		FROM verifications v
		JOIN verification_rows vr ON v.id = vr.verification_id
		WHERE v.date >= ? AND v.date <= ? AND v.type != 'IB'
		ORDER BY v.id, vr.id
	`, fy.StartDate, fy.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch verifications for sie: %w", err)
	}
	defer vRows.Close()

	var currentVid int64 = -1
	for vRows.Next() {
		var vid int64
		var vdate, vtext, account string
		var debet, kredit int64
		
		if err := vRows.Scan(&vid, &vdate, &vtext, &account, &debet, &kredit); err != nil {
			return nil, err
		}

		if vid != currentVid {
			// Close previous block
			if currentVid != -1 {
				buf.WriteString("}\r\n")
			}
			dateClean := strings.ReplaceAll(vdate[:10], "-", "")
			buf.WriteString(fmt.Sprintf("#VER A %d %s \"%s\"\r\n{\r\n", vid, dateClean, sanitize(vtext)))
			currentVid = vid
		}

		// Row balance
		rowBalance := debet - kredit
		rowFloat := float64(rowBalance) / 100.0
		// TRANS syntax: #TRANS kontonr {objektlista} belopp transdat trans_text kvantitet
		buf.WriteString(fmt.Sprintf("    #TRANS %s {} %.2f\r\n", account, rowFloat))
	}
	
	if err = vRows.Err(); err != nil {
		return nil, fmt.Errorf("verification iteration error: %w", err)
	}

	// Close last block
	if currentVid != -1 {
		buf.WriteString("}\r\n")
	}

	// 5. Convert string buffer to PC8 (CP850)
	utf8Str := buf.String()
	encoder := charmap.CodePage850.NewEncoder()
	pc8Bytes, err := encoder.String(utf8Str)
	if err != nil {
		return nil, fmt.Errorf("failed to encode to PC8: %w", err)
	}

	return []byte(pc8Bytes), nil
}
