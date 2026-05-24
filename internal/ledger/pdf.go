package ledger

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"localledger/internal/models"
	"math"
	"os"

	"github.com/go-pdf/fpdf"
)

// GenerateInvoicePDF ritar upp fakturan och returnerar den som en Base64-sträng
func GenerateInvoicePDF(inv models.Invoice, settings models.CompanySettings, invoiceNumber string) (string, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 12)
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	// --- Header ---
	// Logo if exists (Safe handling: if it fails, we draw the fallback logo)
	logoDrawn := false
	if settings.LogoPath != "" {
		if _, err := os.Stat(settings.LogoPath); err == nil {
			// We only warn, not crash if ImageOptions fails internally
			pdf.ImageOptions(settings.LogoPath, 10, 10, 40, 0, false, fpdf.ImageOptions{ReadDpi: true}, 0, "")
			if pdf.Err() {
				pdf.ClearError()
			} else {
				logoDrawn = true
			}
		}
	}

	if !logoDrawn {
		// Draw 3 layered vector diamonds as LocalLedger logo fallback in clean cyan (#06b6d4)
		pdf.SetDrawColor(6, 182, 212)
		pdf.SetLineWidth(0.8)

		// Top Diamond
		points := []fpdf.PointType{
			{X: 25, Y: 9},
			{X: 35, Y: 14},
			{X: 25, Y: 19},
			{X: 15, Y: 14},
		}
		pdf.Polygon(points, "D")

		// Middle V
		pdf.Line(15, 19, 25, 24)
		pdf.Line(25, 24, 35, 19)

		// Bottom V
		pdf.Line(15, 24, 25, 29)
		pdf.Line(25, 29, 35, 24)

		// Reset draw properties to default
		pdf.SetDrawColor(0, 0, 0)
		pdf.SetLineWidth(0.2)
	}

	// Company Name Top Right
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(190, 8, tr(settings.Name), "", 1, "R", false, 0, "")
	pdf.SetFont("Helvetica", "B", 24)
	if inv.TotalAmount < 0 || inv.CreditOf != nil {
		pdf.CellFormat(190, 12, "KREDITFAKTURA", "", 1, "R", false, 0, "")
	} else {
		pdf.CellFormat(190, 12, "FAKTURA", "", 1, "R", false, 0, "")
	}
	
	// Invoice Meta
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(190, 5, fmt.Sprintf("Fakturanummer: %s", invoiceNumber), "", 1, "R", false, 0, "")
	pdf.CellFormat(190, 5, fmt.Sprintf("Fakturadatum: %s", inv.Date), "", 1, "R", false, 0, "")
	if inv.TotalAmount >= 0 {
		pdf.CellFormat(190, 5, fmt.Sprintf("F%srfallodatum: %s", string([]byte{246}), inv.DueDate), "", 1, "R", false, 0, "")
	}

	pdf.Ln(20)

	// --- Customer Details ---
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(100, 6, "Faktureras till:", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	
	pdf.CellFormat(100, 5, tr(inv.CustomerName), "", 1, "L", false, 0, "")
	if inv.CustomerOrgnr != "" {
		pdf.CellFormat(100, 5, fmt.Sprintf("Org.nr: %s", inv.CustomerOrgnr), "", 1, "L", false, 0, "")
	}
	pdf.CellFormat(100, 5, tr(inv.CustomerAddress), "", 1, "L", false, 0, "")

	pdf.Ln(15)

	// --- Invoice Items Table ---
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetFillColor(240, 240, 240)
	pdf.CellFormat(80, 8, "Beskrivning", "1", 0, "L", true, 0, "")
	pdf.CellFormat(25, 8, "Antal", "1", 0, "R", true, 0, "")
	pdf.CellFormat(30, 8, "A-pris", "1", 0, "R", true, 0, "")
	pdf.CellFormat(20, 8, "Moms", "1", 0, "R", true, 0, "")
	pdf.CellFormat(35, 8, "Belopp", "1", 1, "R", true, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	for _, item := range inv.Items {
		// Pagination Logic
		if pdf.GetY() > 240 {
			pdf.AddPage()
			pdf.SetFont("Helvetica", "B", 10)
			pdf.SetFillColor(240, 240, 240)
			pdf.CellFormat(80, 8, "Beskrivning", "1", 0, "L", true, 0, "")
			pdf.CellFormat(25, 8, "Antal", "1", 0, "R", true, 0, "")
			pdf.CellFormat(30, 8, "A-pris", "1", 0, "R", true, 0, "")
			pdf.CellFormat(20, 8, "Moms", "1", 0, "R", true, 0, "")
			pdf.CellFormat(35, 8, "Belopp", "1", 1, "R", true, 0, "")
			pdf.SetFont("Helvetica", "", 10)
		}

		quantityStr := fmt.Sprintf("%.2f", float64(item.Quantity)/100.0)
		priceStr := fmt.Sprintf("%.2f", float64(item.PriceExVat)/100.0)
		
		lineExVatFloat := (float64(item.PriceExVat) * float64(item.Quantity)) / 100.0
		lineExVat := int64(math.Round(lineExVatFloat))
		lineTotalStr := fmt.Sprintf("%.2f", float64(lineExVat)/100.0)
		
		pdf.CellFormat(80, 8, tr(item.Description), "1", 0, "L", false, 0, "")
		pdf.CellFormat(25, 8, quantityStr, "1", 0, "R", false, 0, "")
		pdf.CellFormat(30, 8, priceStr, "1", 0, "R", false, 0, "")
		if item.VatRate == 0 {
			pdf.CellFormat(20, 8, "Momsfritt", "1", 0, "R", false, 0, "")
		} else {
			pdf.CellFormat(20, 8, fmt.Sprintf("%d%%", item.VatRate), "1", 0, "R", false, 0, "")
		}
		pdf.CellFormat(35, 8, lineTotalStr, "1", 1, "R", false, 0, "")
	}

	pdf.Ln(10)

	// --- Totals ---
	var totalExVat int64
	var totalVat int64
	vatBreakdown := make(map[int]int64)

	for _, item := range inv.Items {
		lineExVatFloat := (float64(item.PriceExVat) * float64(item.Quantity)) / 100.0
		lineExVat := int64(math.Round(lineExVatFloat))
		
		lineVatFloat := float64(lineExVat) * float64(item.VatRate) / 100.0
		lineVat := int64(math.Round(lineVatFloat))

		totalExVat += lineExVat
		totalVat += lineVat
		vatBreakdown[item.VatRate] += lineVat
	}
	grandTotal := totalExVat + totalVat

	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(155, 6, "Summa exkl. moms:", "", 0, "R", false, 0, "")
	pdf.CellFormat(35, 6, fmt.Sprintf("%.2f kr", float64(totalExVat)/100.0), "", 1, "R", false, 0, "")

	rates := []int{25, 12, 6, 0}
	for _, rate := range rates {
		if amount := vatBreakdown[rate]; amount != 0 {
			pdf.CellFormat(155, 6, fmt.Sprintf("Moms %d%%:", rate), "", 0, "R", false, 0, "")
			pdf.CellFormat(35, 6, fmt.Sprintf("%.2f kr", float64(amount)/100.0), "", 1, "R", false, 0, "")
		}
	}


	pdf.SetFont("Helvetica", "B", 12)
	if inv.TotalAmount < 0 || inv.CreditOf != nil {
		pdf.CellFormat(155, 8, "Er tillgodo:", "", 0, "R", false, 0, "")
		// Visa beloppet som positivt för "Er tillgodo:"
		pdf.CellFormat(35, 8, fmt.Sprintf("%.2f kr", float64(-grandTotal)/100.0), "", 1, "R", false, 0, "")
	} else {
		pdf.CellFormat(155, 8, "Att betala:", "", 0, "R", false, 0, "")
		pdf.CellFormat(35, 8, fmt.Sprintf("%.2f kr", float64(grandTotal)/100.0), "", 1, "R", false, 0, "")
	}

	// --- Footer ---
	pdf.SetY(-40)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(128, 128, 128)
	
	pdf.CellFormat(63, 5, tr(settings.Name), "", 0, "L", false, 0, "")
	pdf.CellFormat(63, 5, fmt.Sprintf("Org.nr: %s", settings.OrgNumber), "", 0, "C", false, 0, "")
	pdf.CellFormat(64, 5, fmt.Sprintf("Bankgiro: %s", settings.Bankgiro), "", 1, "R", false, 0, "")
	
	pdf.CellFormat(63, 5, tr(settings.Address), "", 0, "L", false, 0, "")
	if settings.SwishNumber != "" {
		pdf.CellFormat(63, 5, "", "", 0, "C", false, 0, "")
		pdf.CellFormat(64, 5, fmt.Sprintf("Swish: %s", settings.SwishNumber), "", 1, "R", false, 0, "")
	} else {
		pdf.CellFormat(63, 5, "", "", 0, "C", false, 0, "")
		pdf.CellFormat(64, 5, "", "", 1, "R", false, 0, "")
	}

	if inv.TotalAmount >= 0 && inv.CreditOf == nil {
		pdf.CellFormat(190, 5, fmt.Sprintf("Betalningsvillkor: %d dagar", settings.PaymentTermsDays), "", 1, "R", false, 0, "")
	} else {
		pdf.CellFormat(190, 5, "Regleras enligt överenskommelse", "", 1, "R", false, 0, "")
	}

	// Output Base64
	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		return "", fmt.Errorf("failed to generate pdf: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
