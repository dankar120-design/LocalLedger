package ocr

import (
	"testing"
)

func TestParseOCRText_Level1_Swedish(t *testing.T) {
	rawText := `ICA NÄRA STOCKHOLM
GUSTAVS VÄG 12
114 30 STOCKHOLM
TEL: 08-123 45 67
ORG NR: 556789-0123
MOMSREH NR: SE556789012301
KVITTO / RECEIPT
DATUM: 2023-11-15 TID: 14:38:21
KVITTONR: 87654 KASSA: 03
ANTAL ARTIKEL PRIS/ST BELOPP (SEK)
1 MJÖLK MELLANL. 1L 18.90 18.90
1 ICA SMÖR 500G 54.50 54.50
2 EKOL. BANANER 1KG 29.90 59.80
1 LÖSVIKTSGODIS 450G 99.00 99.00
1 GROVT RÅGBRÖD 600G 32.90 32.90
3 KEX CHOKLAD (ST) 9.90 29.70
1 REDO-PÅSE PLAST 2.00 2.00
TOTALT: SEK 296.80
MOMS-SPECIFIKATION:
SATTS NETTO MOMS TOTALT
12.003 196.43 23.57 220.00
25.003 61.44 15.36 76.80
TOTALT 257.87 38.93 296.809
BETALAT: KORT (MASTERCARD) SEK 296.80
REFNR: 1029384756 AID: A0000000041010
KORTNR: obbok aobbok sobk 4321
BONUSPOÄNG IDAG: 297
AKTUELLT POÄNGSALDO: 4521
- TACK FÖR BESÖKET!
VÄLKOMMEN ÅTER TILL ICA NÄRA STOCKHOLM`

	res := ParseOCRText(rawText, nil)
	if res.Date != "2023-11-15" {
		t.Errorf("Expected date 2023-11-15, got %s", res.Date)
	}
	// Note: 296.809 will now not match the number regex (\d+[.,]\d{1,2}).
	// So it will fallback to "TOTALT: SEK 296.80" which matches perfectly!
	if res.AmountCents != 29680 {
		t.Errorf("Expected amount 29680, got %d", res.AmountCents)
	}
	if res.Currency != "SEK" {
		t.Errorf("Expected currency SEK, got %s", res.Currency)
	}
}

func TestParseOCRText_Level2_US(t *testing.T) {
	rawText := `SÄ SS S
DAILY
BREAD
S CAFE
DAILY BREAD CAFE
SS 123 Oak Street, Portland, OR 97209
Tel: (503) 555-0199
-- SALES RECEIPT --
> RES: 1 FILL: 3 TRAN: 45678
DATE: OCT 14, 2023 10:45 AM
CASHIER: SARAH J.
1 Latte (Medium) $ 4.75
1 Avocado Toast $ 9.50
1 Blueberry Muffin $ 3.25
1 Drip Coffee (Sm) $ 3.00
SUBTOTAL: $ 20.50
TAX (7.54): $ 1.54
TOTAL: $ 22.04
PAID (CASH): $ 25.00
CHANGE: $ 2.96
Thank You for Your Visit!
Items: 4
wwwW.dailybreadcafe.com
Follow us Gdailybreadcafe d
J`

	res := ParseOCRText(rawText, nil)
	if res.Date != "2023-10-14" {
		t.Errorf("Expected date 2023-10-14, got %s", res.Date)
	}
	// "SUBTOTAL: $ 20.50" should be skipped, "TOTAL: $ 22.04" should win
	if res.AmountCents != 2204 {
		t.Errorf("Expected amount 2204, got %d", res.AmountCents)
	}
	if res.Currency != "FOREIGN" {
		t.Errorf("Expected currency FOREIGN, got %s", res.Currency)
	}
}

func TestParseOCRText_Level5_GarbledUS(t *testing.T) {
	rawText := `STARBUCKS COFFEE 2413. Er
05/18/23 syr |A MR
REG: 3 NV a fr"
SUBTOTALAFJ ar ES [FE
TAX Få NE sä rg
TOTAE Y 4 AR RV 3 T`

	res := ParseOCRText(rawText, nil)
	// Test 2-digit US date regex
	if res.Date != "2023-05-18" {
		t.Errorf("Expected date 2023-05-18, got %s", res.Date)
	}
}

func TestParseOCRText_Level1_ThousandSeparator(t *testing.T) {
	rawText := `WEBHALLEN SVERIGE AB
ORG NR: 556789-0123
KVITTO
DATUM: 2023-11-15
1 GAMING LAPTOP 12 450,50
TOTALT: SEK 12 450,50
MOMS 25% 2 490,10`

	res := ParseOCRText(rawText, nil)
	if res.AmountCents != 1245050 {
		t.Errorf("Expected amount 1245050, got %d", res.AmountCents)
	}
}

func TestParseOCRText_Level1_PhoneNumberTrap(t *testing.T) {
	rawText := `BUTIKEN AB
DATUM: 2023-11-15
TOTALT: 296.80 TEL 08-123 45.67`

	// The old regex "\d[\d\s]*[.,]\d{1,2}\b" would match "123 45.67" as the last number and return 1234567.
	// The new strict regex "\d{1,3}(?:\s\d{3})*[.,]\d{1,2}\b" should reject "123 45.67" because "45" is not a 3-digit group.
	// It should instead match "296.80" and return 29680.
	res := ParseOCRText(rawText, nil)
	if res.AmountCents != 29680 {
		t.Errorf("Expected amount 29680, got %d", res.AmountCents)
	}
}

func TestParseOCRText_Slice3_InvertedMatching(t *testing.T) {
	rawText := `ICA NARA STOCKHOLM
GUSTAVS VÄG 12
114 30 STOCKHOLM
TEL: 08-123 45 67
ORG NR: 556789-0123
MOMSREH NR: SE556789012301
KVITTO / RECEIPT
DATUM: 2023-11-15 TID: 14:38:21
KVITTONR: 87654 KASSA: 03
ANTAL ARTIKEL PRIS/ST BELOPP (SEK)
1 MJÖLK MELLANL. 1L 18.90 18.90
1 KAFFE ZOEGAS 500G 54.50 54.50
TOTALT: SEK 73.40`

	knownVendors := []string{"ICA", "ICA NÄRA STOCKHOLM", "COOP", "KAFFE"}

	// "KAFFE" should NOT be matched because it's on line 12 (outside Top 10)
	// "ICA" and "ICA NÄRA STOCKHOLM" both match (ICA NARA gets normalized to ICA NARA, matching ICA NARA).
	// "ICA NÄRA STOCKHOLM" is longer, so it should win.
	res := ParseOCRText(rawText, knownVendors)

	if res.Vendor != "ICA NÄRA STOCKHOLM" {
		t.Errorf("Expected vendor 'ICA NÄRA STOCKHOLM', got '%s'", res.Vendor)
	}
}

func TestParseOCRText_Slice3_UnicodeAndPerformanceFilter(t *testing.T) {
	rawText := `CAFÉ ROUGE
PARIS VÄG 12
114 30 STOCKHOLM
TEL: 08-123 45 67
TOTALT: SEK 73.40`

	// "CAFÉ ROUGE" in DB. The OCR text has "CAFÉ ROUGE".
	// Without diacritic replacer, Go's \b would fail because É is \W.
	// We test that it successfully normalizes both to "CAFE ROUGE" and matches.
	knownVendors := []string{"CAFÉ ROUGE", "CAFE", "ROUGE"}

	res := ParseOCRText(rawText, knownVendors)

	if res.Vendor != "CAFÉ ROUGE" {
		t.Errorf("Expected vendor 'CAFÉ ROUGE', got '%s'", res.Vendor)
	}
}

func TestParseOCRText_Slice3_WordBoundaryNegative(t *testing.T) {
	rawText := `AMERICAN DINER
TOTALT: SEK 150.00`

	// "ICA" is a known vendor, but should NOT match inside "AMERICAN DINER"
	knownVendors := []string{"ICA"}
	res := ParseOCRText(rawText, knownVendors)

	if res.Vendor != "" {
		t.Errorf("Expected no vendor match due to word boundaries, got '%s'", res.Vendor)
	}
}
