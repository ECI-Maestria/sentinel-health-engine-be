package pdf

import (
	"bytes"
	"fmt"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/sentinel-health-engine/analytics-service/internal/cosmosdb"
)

const (
	maxVitalsRows = 50
	maxAlertRows  = 20
)

// GeneratePatientReport builds a PDF patient report and returns it as a byte slice.
// It includes a header, a vitals table (up to 50 readings), an alerts table (up to 20
// alerts), and a footer with the generation timestamp.
func GeneratePatientReport(
	patientName string,
	from, to time.Time,
	vitals []cosmosdb.VitalReading,
	alerts []cosmosdb.AlertRecord,
) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	pageW, _ := pdf.GetPageSize()
	contentW := pageW - 30 // left + right margins

	// ── Header ─────────────────────────────────────────────────────────────────
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(contentW, 10, "Sentinel Health Engine — Informe del Paciente", "", 1, "C", false, 0, "")

	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(contentW, 7, fmt.Sprintf("Paciente: %s", patientName), "", 1, "L", false, 0, "")
	pdf.CellFormat(contentW, 7,
		fmt.Sprintf("Período: %s — %s",
			from.UTC().Format("02/01/2006"),
			to.UTC().Format("02/01/2006"),
		),
		"", 1, "L", false, 0, "")

	pdf.Ln(4)

	// ── Vitals section ─────────────────────────────────────────────────────────
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(contentW, 8, "Signos Vitales", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// Table header
	colWidths := []float64{55, 65, 55}
	headers := []string{"Fecha", "Frecuencia Cardíaca (bpm)", "SpO2 (%)"}

	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetFillColor(200, 220, 255)
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 7, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	limit := len(vitals)
	if limit > maxVitalsRows {
		limit = maxVitalsRows
	}

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetFillColor(240, 240, 240)
	for i := 0; i < limit; i++ {
		v := vitals[i]
		fill := i%2 == 1
		pdf.CellFormat(colWidths[0], 6, v.MeasuredAt.UTC().Format("02/01/2006 15:04"), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colWidths[1], 6, fmt.Sprintf("%d", v.HeartRate), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colWidths[2], 6, fmt.Sprintf("%.1f", v.SpO2), "1", 0, "C", fill, 0, "")
		pdf.Ln(-1)
	}

	if len(vitals) == 0 {
		pdf.SetFont("Helvetica", "I", 10)
		pdf.CellFormat(contentW, 6, "Sin lecturas en el período seleccionado.", "1", 1, "C", false, 0, "")
	}

	pdf.Ln(6)

	// ── Alerts section ─────────────────────────────────────────────────────────
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(contentW, 8, "Alertas", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	alertColWidths := []float64{50, 35, 90}
	alertHeaders := []string{"Fecha", "Severidad", "Mensaje"}

	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetFillColor(200, 220, 255)
	for i, h := range alertHeaders {
		pdf.CellFormat(alertColWidths[i], 7, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	alertLimit := len(alerts)
	if alertLimit > maxAlertRows {
		alertLimit = maxAlertRows
	}

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetFillColor(255, 230, 230)
	for i := 0; i < alertLimit; i++ {
		a := alerts[i]
		fill := a.Severity == "CRITICAL"

		// Truncate long messages to avoid overflowing cells
		msg := a.Message
		if len(msg) > 60 {
			msg = msg[:57] + "..."
		}

		pdf.CellFormat(alertColWidths[0], 6, a.CreatedAt.UTC().Format("02/01/2006 15:04"), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(alertColWidths[1], 6, a.Severity, "1", 0, "C", fill, 0, "")
		pdf.CellFormat(alertColWidths[2], 6, msg, "1", 0, "L", fill, 0, "")
		pdf.Ln(-1)
	}

	if len(alerts) == 0 {
		pdf.SetFont("Helvetica", "I", 10)
		pdf.CellFormat(contentW, 6, "Sin alertas en el período seleccionado.", "1", 1, "C", false, 0, "")
	}

	// ── Footer ─────────────────────────────────────────────────────────────────
	pdf.SetFont("Helvetica", "I", 8)
	_, pageH := pdf.GetPageSize()
	pdf.SetY(pageH - 15)
	pdf.CellFormat(contentW, 5,
		fmt.Sprintf("Generado el %s UTC", time.Now().UTC().Format("02/01/2006 15:04:05")),
		"", 0, "C", false, 0, "")

	if pdf.Error() != nil {
		return nil, fmt.Errorf("building PDF: %w", pdf.Error())
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("writing PDF to buffer: %w", err)
	}

	return buf.Bytes(), nil
}
