package reporting

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/gophish/gophish/models"
	"github.com/jung-kurt/gofpdf"
)

// GenerateScanReport produces a professional PDF report for a given scan task.
func GenerateScanReport(scanID uint, userID int64) ([]byte, error) {
	scan, err := models.GetScanTask(userID, scanID)
	if err != nil {
		return nil, fmt.Errorf("getting scan task: %w", err)
	}

	findings, err := models.GetFindingsForScan(userID, scanID)
	if err != nil {
		return nil, fmt.Errorf("getting findings: %w", err)
	}

	stats, err := models.GetScanFindingStats(userID, scanID)
	if err != nil {
		stats = map[string]int64{} // fallback
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// --- Header ---
	pdf.SetFillColor(11, 15, 18) // Vantage Background Color
	pdf.Rect(0, 0, 210, 40, "F")
	
	pdf.SetTextColor(94, 207, 136) // Vantage Accent Green
	pdf.SetFont("Arial", "B", 24)
	pdf.CellFormat(0, 15, "VANTAGE SECURITY REPORT", "", 1, "C", false, 0, "")
	
	pdf.SetTextColor(200, 200, 200)
	pdf.SetFont("Arial", "I", 10)
	pdf.CellFormat(0, 5, fmt.Sprintf("Report Generated: %s", time.Now().Format("2006-01-02 15:04:05")), "", 1, "C", false, 0, "")
	pdf.Ln(15)

	// --- Executive Summary ---
	pdf.SetTextColor(50, 50, 50)
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "1. Executive Summary")
	pdf.Ln(10)
	
	pdf.SetFont("Arial", "", 11)
	summary := fmt.Sprintf("This document contains the security assessment results for target: %s. "+
		"The scan was performed using the Vantage Security Hub using various ProjectDiscovery tools. "+
		"A total of %d findings were identified during this assessment.", scan.Target, len(findings))
	pdf.MultiCell(0, 6, summary, "", "", false)
	pdf.Ln(10)

	// --- Scan Metadata ---
	pdf.SetFillColor(240, 240, 240)
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(40, 8, "Target", "1", 0, "L", true, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(0, 8, scan.Target, "1", 1, "L", false, 0, "")
	
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(40, 8, "Scan Mode", "1", 0, "L", true, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(0, 8, scan.Mode, "1", 1, "L", false, 0, "")

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(40, 8, "Interface", "1", 0, "L", true, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(0, 8, scan.OutboundInterface, "1", 1, "L", false, 0, "")

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(40, 8, "Duration", "1", 0, "L", true, 0, "")
	pdf.SetFont("Arial", "", 11)
	duration := "N/A"
	if !scan.UpdatedAt.IsZero() {
		duration = scan.UpdatedAt.Sub(scan.CreatedAt).String()
	}
	pdf.CellFormat(0, 8, duration, "1", 1, "L", false, 0, "")
	pdf.Ln(10)

	// --- Severity Breakdown ---
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "2. Risk Distribution")
	pdf.Ln(10)

	severities := []string{"critical", "high", "medium", "low", "info"}
	colors := map[string][]int{
		"critical": {188, 140, 242},
		"high":     {248, 81, 73},
		"medium":   {210, 153, 34},
		"low":      {139, 148, 158},
		"info":     {88, 166, 255},
	}

	for _, sev := range severities {
		count := stats[sev]
		c := colors[sev]
		pdf.SetFillColor(c[0], c[1], c[2])
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Arial", "B", 11)
		pdf.CellFormat(30, 8, strings.ToUpper(sev), "1", 0, "C", true, 0, "")
		
		pdf.SetTextColor(50, 50, 50)
		pdf.SetFont("Arial", "", 11)
		pdf.CellFormat(20, 8, fmt.Sprintf("%d", count), "1", 1, "C", false, 0, "")
	}
	pdf.Ln(15)

	// --- Detailed Findings ---
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "3. Detailed Findings")
	pdf.Ln(10)

	for i, f := range findings {
		// New page if near bottom
		if pdf.GetY() > 250 {
			pdf.AddPage()
		}

		pdf.SetFont("Arial", "B", 12)
		pdf.SetFillColor(230, 230, 230)
		pdf.CellFormat(0, 10, fmt.Sprintf("%d. %s", i+1, f.Name), "T", 1, "L", true, 0, "")
		
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(30, 8, "Severity:")
		pdf.SetFont("Arial", "", 10)
		c := colors[f.Severity]
		pdf.SetTextColor(c[0], c[1], c[2])
		pdf.Cell(40, 8, strings.ToUpper(f.Severity))
		pdf.SetTextColor(50, 50, 50)
		
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(20, 8, "Tool:")
		pdf.SetFont("Arial", "", 10)
		pdf.Cell(0, 8, f.ToolName)
		pdf.Ln(8)

		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(30, 8, "Target:")
		pdf.SetFont("Arial", "", 10)
		pdf.Cell(0, 8, f.Target)
		pdf.Ln(8)

		if f.TemplateID != "" {
			pdf.SetFont("Arial", "B", 10)
			pdf.Cell(30, 8, "Template:")
			pdf.SetFont("Arial", "", 10)
			pdf.Cell(0, 8, f.TemplateID)
			pdf.Ln(8)
		}

		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(0, 8, "Data / Proof:")
		pdf.Ln(6)
		pdf.SetFont("Courier", "", 8)
		pdf.MultiCell(0, 4, f.Detail, "1", "L", false)
		pdf.Ln(5)
	}

	// --- Footer ---
	pdf.SetY(-20)
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(128, 128, 128)
	pdf.CellFormat(0, 10, "Vantage Security Hub - Confidential Technical Report - Page "+fmt.Sprintf("%d", pdf.PageNo()), "", 0, "C", false, 0, "")

	var buf bytes.Buffer
	err = pdf.Output(&buf)
	return buf.Bytes(), err
}
