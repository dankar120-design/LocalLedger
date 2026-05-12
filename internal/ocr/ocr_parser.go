package ocr

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type OCRResult struct {
	Date        string `json:"date"`
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
	Vendor      string `json:"vendor,omitempty"`
}

// Globala regex-motorer (kompileras endast en gång vid uppstart)
var (
	isoRe             = regexp.MustCompile(`\b(20\d{2}|\d{2})[\-\.](0[1-9]|1[0-2])[\-\.](0[1-9]|[12]\d|3[01])`)
	usRe              = regexp.MustCompile(`\b(0[1-9]|1[0-2])/([0-2]\d|3[0-1])/(\d{2,4})`)
	txtRe             = regexp.MustCompile(`(?i)\b(JAN|FEB|MAR|APR|MAY|JUN|JUL|AUG|SEP|OCT|NOV|DEC)[A-Z]*\s+(\d{1,2})[\s,]+(\d{4})`)
	foreignCurrencyRe = regexp.MustCompile(`(?i)\b(USD|EUR)\b|\$|€`)
	subtotalRe        = regexp.MustCompile(`(?i)SUB[\s\-]*TOTAL`)
	// Belopp kräver explicit en decimalavskiljare följt av 1-2 decimaler för att undvika falska träffar på t.ex. telefonnummer.
	// Tillåter endast strikta tusentalsgrupperingar (ex: 1 250,00 eller 12 450,00) för att inte fånga upp telefonnummer som "08-123 45.67"
	numberRe = regexp.MustCompile(`\d{1,3}(?:\s\d{3})*[.,]\d{1,2}\b`)

	// Normaliserare för europeiska och svenska tecken (Tesseract misstolkar ofta dessa, och Go's \b hanterar inte unicode).
	// Vi konverterar texten till versaler innan denna körs, så vi behöver bara lista versaler.
	diacriticReplacer = strings.NewReplacer(
		"Å", "A", "Ä", "A", "Ö", "O",
		"É", "E", "È", "E", "Ü", "U",
		"Ç", "C", "Ñ", "N",
	)
)

// ParseOCRText takes raw text from Tesseract and known vendors from DB, and returns a structured OCRResult
func ParseOCRText(rawText string, knownVendors []string) OCRResult {
	result := OCRResult{
		Currency: "SEK", // Default
	}

	result.Vendor = extractVendor(rawText, knownVendors)
	result.Date = extractDate(rawText)

	amount, currency := extractAmountAndCurrency(rawText)
	result.AmountCents = amount

	if currency != "" {
		result.Currency = currency
	}

	return result
}

func extractVendor(rawText string, knownVendors []string) string {
	if len(knownVendors) == 0 {
		return ""
	}

	lines := strings.Split(rawText, "\n")
	
	// Top-Heavy Search: Företagsnamnet står i headern. 
	// Vi letar bara i de 10 första raderna för att undvika falska träffar på varor längre ner.
	maxLines := 10
	if len(lines) < maxLines {
		maxLines = len(lines)
	}
	headerText := strings.Join(lines[:maxLines], "\n")

	// Normalisera texten för matchning
	headerUpper := strings.ToUpper(headerText)
	headerNorm := diacriticReplacer.Replace(headerUpper)

	var bestVendor string
	
	for _, vendor := range knownVendors {
		vendorUpper := strings.ToUpper(vendor)
		vendorNorm := diacriticReplacer.Replace(vendorUpper)

		// Prestandafilter (Snabbspår): Undvik att kompilera tunga regex om vi inte ens har en substring-träff
		if !strings.Contains(headerNorm, vendorNorm) {
			continue
		}

		// Skapa regex med ordgränser (escape vendor-namnet först ifall det innehåller specialtecken)
		escapedVendor := regexp.QuoteMeta(vendorNorm)
		
		// Regex för att hitta vendorn med ordgränser.
		// Eftersom vi nu normaliserat bort diakritiska tecken, fungerar Go's \b (ASCII-ordgräns) perfekt.
		pattern := `\b` + escapedVendor + `\b`
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue // Om kompilering misslyckas (ovanligt med QuoteMeta), hoppa över
		}

		if re.MatchString(headerNorm) {
			// Om vi hittar en match, och det här ursprungliga vendor-namnet är längre
			// än det vi eventuellt redan hittat (för att välja "ICA NÄRA" istället för bara "ICA"),
			// så sparar vi det.
			if len(vendor) > len(bestVendor) {
				bestVendor = vendor
			}
		}
	}

	return bestVendor
}

func extractDate(text string) string {
	// 1. ISO-ish (e.g. 2023-11-15, 2023.11.15, 23-11-15)
	if match := isoRe.FindStringSubmatch(text); match != nil {
		year := match[1]
		if len(year) == 2 {
			year = "20" + year
		}
		return fmt.Sprintf("%s-%s-%s", year, match[2], match[3])
	}

	// 2. US/Slash (e.g. 10/26/2023, 05/18/23)
	if match := usRe.FindStringSubmatch(text); match != nil {
		month := match[1]
		day := match[2]
		year := match[3]
		if len(year) == 2 {
			year = "20" + year
		}
		return fmt.Sprintf("%s-%s-%s", year, month, day)
	}

	// 3. Text Month (e.g. OCT 14, 2023)
	if match := txtRe.FindStringSubmatch(text); match != nil {
		monthStr := strings.ToUpper(match[1][:3])
		dayStr := match[2]
		if len(dayStr) == 1 {
			dayStr = "0" + dayStr
		}
		yearStr := match[3]

		months := map[string]string{
			"JAN": "01", "FEB": "02", "MAR": "03", "APR": "04",
			"MAY": "05", "JUN": "06", "JUL": "07", "AUG": "08",
			"SEP": "09", "OCT": "10", "NOV": "11", "DEC": "12",
		}
		return fmt.Sprintf("%s-%s-%s", yearStr, months[monthStr], dayStr)
	}

	return ""
}

func extractAmountAndCurrency(text string) (int64, string) {
	lines := strings.Split(text, "\n")

	keywords := []string{"TOTAL", "TOTALT", "SUMMA", "BELOPP", "ATT BETALA"}

	var bestAmount int64 = 0
	var foundCurrency = ""

	for i, line := range lines {
		// Hoppa över rader som är SUBTOTAL
		if subtotalRe.MatchString(line) {
			continue
		}

		lineUpper := strings.ToUpper(line)
		hasKeyword := false
		for _, kw := range keywords {
			if strings.Contains(lineUpper, kw) {
				hasKeyword = true
				break
			}
		}

		if hasKeyword {
			// Check for foreign currency on this line and neighbors
			if foreignCurrencyRe.MatchString(line) {
				foundCurrency = "FOREIGN"
			} else {
				if i > 0 && foreignCurrencyRe.MatchString(lines[i-1]) {
					foundCurrency = "FOREIGN"
				}
				if i < len(lines)-1 && foreignCurrencyRe.MatchString(lines[i+1]) {
					foundCurrency = "FOREIGN"
				}
			}

			// Extract all numbers on the line
			matches := numberRe.FindAllString(line, -1)
			for _, match := range matches {
				val, err := normalizeAmount(match)
				if err == nil && val > bestAmount {
					bestAmount = val // Sparar det största beloppet på raden
				}
			}
		}
	}

	if foundCurrency != "" {
		return bestAmount, foundCurrency
	}
	return bestAmount, "SEK"
}

func normalizeAmount(s string) (int64, error) {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", ".")

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}

	return int64(val*100 + 0.5), nil
}
