// Package render builds the virtual thermal-paper lines for fiscal receipts,
// credit/debit notes, non-fiscal documents and X/Z reports.
//
// Every line is at most PAPER_WIDTH code points so the web panel can render
// them in a monospace "paper roll" exactly like a 40-column thermal printer.
package render

import (
	"fmt"
	"strings"
	"time"

	"simuladorfiscalgo/internal/orderedmap"
	"simuladorfiscalgo/internal/pyfmt"
	"simuladorfiscalgo/internal/state"
)

// PAPER_WIDTH is the printable width of the virtual roll.
const PAPER_WIDTH = 42

var units = []string{
	"", "UNO", "DOS", "TRES", "CUATRO", "CINCO", "SEIS", "SIETE",
	"OCHO", "NUEVE", "DIEZ", "ONCE", "DOCE", "TRECE", "CATORCE",
	"QUINCE", "DIECISÉIS", "DIECISIETE", "DIECIOCHO", "DIECINUEVE",
	"VEINTE", "VEINTIUNO", "VEINTIDÓS", "VEINTITRÉS", "VEINTICUATRO",
	"VEINTICINCO", "VEINTISÉIS", "VEINTISIETE", "VEINTIOCHO",
	"VEINTINUEVE",
}

var tens = []string{"", "", "", "TREINTA", "CUARENTA", "CINCUENTA", "SESENTA",
	"SETENTA", "OCHENTA", "NOVENTA"}

var hundreds = []string{"", "CIENTO", "DOSCIENTOS", "TRESCIENTOS", "CUATROCIENTOS",
	"QUINIENTOS", "SEISCIENTOS", "SETECIENTOS", "OCHOCIENTOS", "NOVECIENTOS"}

func numToWordsUpTo999(n int) string {
	if n == 0 {
		return ""
	}
	if n == 100 {
		return "CIEN"
	}
	h := n / 100
	rest := n % 100
	var parts []string
	if h > 0 {
		parts = append(parts, hundreds[h])
	}
	if rest > 0 {
		if rest < 30 {
			parts = append(parts, units[rest])
		} else {
			t := rest / 10
			u := rest % 10
			word := tens[t]
			if u > 0 {
				word += " Y " + units[u]
			}
			parts = append(parts, word)
		}
	}
	return strings.Join(parts, " ")
}

// NumberToWords converts an amount to Spanish words, e.g. 1234.50 ->
// "MIL DOSCIENTOS TREINTA Y CUATRO CON 50/100". Required by law on every
// receipt ("SON: ...").
func NumberToWords(amount float64) string {
	if amount < 0 {
		amount = -amount
	}
	whole := int(amount)
	cents := int(pyfmt.PyRound((amount-float64(whole))*100, 0))
	if cents == 100 {
		whole++
		cents = 0
	}

	var words string
	if whole == 0 {
		words = "CERO"
	} else {
		var chunks []string
		millions := whole / 1_000_000
		rest := whole % 1_000_000
		thousands := rest / 1000
		unitsPart := rest % 1000
		if millions > 0 {
			if millions == 1 {
				chunks = append(chunks, "UN MILLÓN")
			} else {
				chunks = append(chunks, numToWordsUpTo999(millions)+" MILLONES")
			}
		}
		if thousands > 0 {
			if thousands == 1 {
				chunks = append(chunks, "MIL")
			} else {
				chunks = append(chunks, numToWordsUpTo999(thousands)+" MIL")
			}
		}
		if unitsPart > 0 {
			chunks = append(chunks, numToWordsUpTo999(unitsPart))
		}
		words = strings.Join(chunks, " ")
	}
	return fmt.Sprintf("%s CON %02d/100", words, cents)
}

// ---------------------------------------------------------------- helpers

func center(text string) string { return pyfmt.Center(text, PAPER_WIDTH) }

func rule(char string) string { return strings.Repeat(char, PAPER_WIDTH) }

// kv puts key+sep on the left and value right-aligned, filling the width.
func kv(key, value, sep string) string {
	left := key + sep + " "
	right := value
	pad := PAPER_WIDTH - pyfmt.Count(left) - pyfmt.Count(right)
	if pad < 1 {
		return pyfmt.Slice(left+right, 0, PAPER_WIDTH)
	}
	return left + strings.Repeat(" ", pad) + right
}

func kvColon(key, value string) string { return kv(key, value, ":") }

func money(v float64) string { return pyfmt.Money(v) }

// wrap folds text into lines of at most `width` code points.
func wrap(text string, width int) []string {
	words := strings.Fields(text)
	var lines []string
	current := ""
	for _, w := range words {
		if current == "" {
			current = w
		} else if pyfmt.Count(current)+1+pyfmt.Count(w) <= width {
			current += " " + w
		} else {
			lines = append(lines, current)
			current = w
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func companyBlock(c state.Company) []string {
	var lines []string
	name := c.RazonSocial
	if name == "" {
		name = "EMPRESA SIN NOMBRE"
	}
	lines = append(lines, center(strings.ToUpper(name)))
	if c.Rif != "" {
		lines = append(lines, center("RIF: "+c.Rif))
	}
	if c.LogoText != "" && c.LogoText != name {
		lines = append(lines, center(strings.ToUpper(c.LogoText)))
	}
	if c.Direccion != "" {
		for _, l := range wrap(c.Direccion, 40) {
			lines = append(lines, center(l))
		}
	}
	locality := joinNonEmpty([]string{c.Municipio, c.Ciudad, c.Estado}, " - ")
	if locality != "" {
		for _, l := range wrap(locality, 40) {
			lines = append(lines, center(l))
		}
	}
	if c.Telefono != "" {
		lines = append(lines, center("Telf: "+c.Telefono))
	}
	if c.RepresentanteLegal != "" {
		rep := "Rep. Legal: " + c.RepresentanteLegal
		if c.CedulaRepresentante != "" {
			rep += " - C.I. " + c.CedulaRepresentante
		}
		for _, l := range wrap(rep, 40) {
			lines = append(lines, center(l))
		}
	}
	if c.ActividadEconomica != "" {
		for _, l := range wrap("Actividad: "+c.ActividadEconomica, 40) {
			lines = append(lines, center(l))
		}
	}
	if c.ContribuyenteEspecial {
		lines = append(lines, center("CONTRIBUYENTE ESPECIAL"))
	}
	if c.AgenteRetencion {
		lines = append(lines, center("AGENTE DE RETENCION IVA"))
	}
	return lines
}

func joinNonEmpty(parts []string, sep string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}

var docTitles = map[string]string{
	"invoice":     "FACTURA FISCAL",
	"credit_note": "NOTA DE CREDITO",
	"debit_note":  "NOTA DE DEBITO",
	"non_fiscal":  "DOCUMENTO NO FISCAL",
}

var taxLabels = map[string]string{
	"general": "G16", "reduced": "R8", "additional": "A30", "exempt": "EX",
}

var paymentLabels = map[string]string{
	"cash": "EFECTIVO", "card": "TARJETA", "transfer": "TRANSFERENCIA",
	"mobile": "PAGO MOVIL", "credit": "CREDITO", "check": "CHEQUE",
	"pay_0": "EFECTIVO", "pay_1": "TARJETA", "pay_2": "TRANSFERENCIA",
}

func paymentLabel(method string) string {
	if l, ok := paymentLabels[method]; ok {
		return l
	}
	return strings.ToUpper(method)
}

// paymentLabelLower mirrors the X/Z report path, which lower-cases the key
// before the dictionary lookup (unlike the receipt path).
func paymentLabelLower(method string) string {
	if l, ok := paymentLabels[strings.ToLower(method)]; ok {
		return l
	}
	return strings.ToUpper(method)
}

func ts(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("02/01/2006  15:04:05")
}

// RenderReceipt renders a closed fiscal document as paper lines.
func RenderReceipt(doc *state.Document, company state.Company) []string {
	var lines []string

	// Header
	lines = append(lines, companyBlock(company)...)
	lines = append(lines, rule("="))
	docType := doc.Type
	if docType == "" {
		docType = "invoice"
	}
	lines = append(lines, center(docTitles[docType]))
	lines = append(lines, center(fmt.Sprintf("N° %08d", doc.Number)))
	if !doc.Timestamp.IsZero() {
		lines = append(lines, center(ts(doc.Timestamp)))
	}
	lines = append(lines, rule("="))

	// Customer
	cust := doc.Customer
	lines = append(lines, "CLIENTE:")
	if cust.Rif != "" {
		lines = append(lines, "  RIF/C.I: "+cust.Rif)
	}
	if cust.Name != "" {
		for _, l := range wrap("  "+cust.Name, 42) {
			lines = append(lines, l)
		}
	}
	if cust.Address != "" {
		for _, l := range wrap("  "+cust.Address, 42) {
			lines = append(lines, l)
		}
	}
	if cust.Rif == "" && cust.Name == "" && cust.Address == "" {
		lines = append(lines, "  CONSUMIDOR FINAL")
	}
	lines = append(lines, rule("-"))

	// Reference block for credit/debit notes
	if docType == "credit_note" || docType == "debit_note" {
		if doc.ReferenceInvoice != "" {
			lines = append(lines, kvColon("FACT. REFERIDA", doc.ReferenceInvoice))
		}
		if doc.ReferenceDate != "" {
			lines = append(lines, kvColon("FECHA ORIG.", doc.ReferenceDate))
		}
		if doc.ReferenceSerial != "" {
			lines = append(lines, kvColon("SERIE ORIG.", doc.ReferenceSerial))
		}
		lines = append(lines, rule("-"))
	}

	// Items table
	lines = append(lines, fmt.Sprintf("%7s %-17s %7s %4s %9s",
		"CANT", "DESCRIPCION", "PREC", "IVA", "IMPORTE"))
	lines = append(lines, rule("-"))
	for _, item := range doc.Items {
		desc := pyfmt.Slice(item.Description, 0, 17)
		label := taxLabels[item.TaxType]
		if label == "" {
			label = "G16"
		}
		lines = append(lines, fmt.Sprintf("%7.3f %-17s %7.2f %4s %9.2f",
			item.Quantity, desc, item.Price, label, item.Total))
		// Long descriptions fold onto extra lines.
		if pyfmt.Count(item.Description) > 17 {
			for _, l := range wrap("   "+item.Description, 42)[1:] {
				lines = append(lines, l)
			}
		}
	}
	lines = append(lines, rule("-"))

	// Totals
	lines = append(lines, kvColon("SUBTOTAL", money(doc.SubtotalBases+doc.SubtotalTax)))

	// Per-rate breakdown
	type block struct{ base, tax float64 }
	byRate := map[string]*block{}
	order := []string{"general", "reduced", "additional", "exempt"}
	for _, item := range doc.Items {
		b, ok := byRate[item.TaxType]
		if !ok {
			b = &block{}
			byRate[item.TaxType] = b
		}
		b.base += item.Base
		b.tax += item.Tax
	}
	pctLabel := map[string]string{
		"general": "16%", "reduced": "8%", "additional": "30%", "exempt": "EXENTO",
	}
	for _, rate := range order {
		b, ok := byRate[rate]
		if !ok {
			continue
		}
		pct := pctLabel[rate]
		if rate == "exempt" {
			lines = append(lines, kvColon("  BASE EXENTA", money(b.base)))
		} else {
			lines = append(lines, kvColon("  BASE IVA "+pct, money(b.base)))
			lines = append(lines, kvColon("  IVA "+pct, money(b.tax)))
		}
	}
	lines = append(lines, rule("="))
	lines = append(lines, kvColon("TOTAL Bs.", money(doc.Total)))
	lines = append(lines, rule("="))

	// Payments
	if doc.Payments != nil {
		doc.Payments.ForEach(func(method string, amount float64) {
			lines = append(lines, kvColon(paymentLabel(method), money(amount)))
		})
	}
	if doc.Change != 0 {
		lines = append(lines, kvColon("CAMBIO", money(doc.Change)))
	}

	// Amount in words (legal requirement)
	lines = append(lines, rule("-"))
	words := NumberToWords(doc.Total)
	for _, l := range wrap("SON: "+words, 42) {
		lines = append(lines, "  "+l)
	}

	// Legal footnotes
	lines = append(lines, rule("-"))
	if doc.SerialNumber != "" {
		lines = append(lines, kvColon("MAQUINA", doc.SerialNumber))
	}
	if doc.Rif != "" {
		lines = append(lines, kvColon("RIF MAQ.", doc.Rif))
	}
	if company.CondicionesPago != "" {
		lines = append(lines, kvColon("COND. PAGO", company.CondicionesPago))
	}
	lines = append(lines, center("ART. 63 Ley IVA - Ley Org."))
	lines = append(lines, center("Simplificacion y Racionalizacion"))
	lines = append(lines, center("del Proceso Tributario"))
	lines = append(lines, rule("-"))
	// Python reads company.get("footers"), which the company dict never has,
	// so the footer always falls back to this default.
	footer := "GRACIAS POR SU COMPRA"
	lines = append(lines, center(footer))
	lines = append(lines, center("* * * * * * * * * * * * * * * *"))
	lines = append(lines, center("DOCUMENTO FISCAL DIGITAL"))
	lines = append(lines, center("SIMULADOR - NO VALOR FISCAL"))
	return lines
}

// ---------------------------------------------------------------- reports

func reportCommonLines(rep *state.Report, company state.Company, title string) []string {
	lines := companyBlock(company)
	lines = append(lines, rule("="))
	lines = append(lines, center(title))
	lines = append(lines, center(fmt.Sprintf("REPORTE N° %04d", rep.Number)))
	if !rep.Timestamp.IsZero() {
		lines = append(lines, center(ts(rep.Timestamp)))
	}
	lines = append(lines, rule("="))
	return lines
}

var reportRateOrder = []string{"general", "reduced", "additional", "exempt"}
var reportRateLabels = map[string]string{
	"general": "GENERAL 16%", "reduced": "REDUCIDA 8%",
	"additional": "ADICIONAL 30%", "exempt": "EXENTO",
}

func reportBodyLines(rep *state.Report) []string {
	lines := []string{"DOCUMENTOS DEL DIA:"}
	lines = append(lines, kvColon("  Facturas", fmt.Sprint(rep.DailyInvoices)))
	lines = append(lines, kvColon("  Notas de credito", fmt.Sprint(rep.DailyCreditNotes)))
	lines = append(lines, kvColon("  Notas de debito", fmt.Sprint(rep.DailyDebitNotes)))
	lines = append(lines, kvColon("  No fiscales", fmt.Sprint(rep.DailyNonFiscal)))
	lines = append(lines, rule("-"))

	lines = append(lines, "VENTAS POR ALICUOTA:")
	for _, rate := range reportRateOrder {
		b, ok := rep.TaxTotals[rate]
		if !ok {
			continue
		}
		if b.Base == 0 && b.Tax == 0 {
			continue
		}
		lines = append(lines, kvColon("  "+reportRateLabels[rate], money(b.Base)))
		if rate != "exempt" {
			lines = append(lines, kvColon("    IVA", money(b.Tax)))
		}
	}
	lines = append(lines, rule("-"))

	lines = append(lines, "FORMAS DE PAGO:")
	if rep.Payments != nil && rep.Payments.Len() > 0 {
		rep.Payments.ForEach(func(method string, amount float64) {
			lines = append(lines, kvColon("  "+paymentLabelLower(method), money(amount)))
		})
	} else {
		lines = append(lines, "  (sin pagos registrados)")
	}
	lines = append(lines, rule("="))

	lines = append(lines, kvColon("TOTAL VENTAS", money(rep.TotalSales)))
	lines = append(lines, kvColon("TOTAL IVA", money(rep.TotalTax)))
	if rep.TotalDiscounts != 0 {
		lines = append(lines, kvColon("DESCUENTOS", money(rep.TotalDiscounts)))
	}
	if rep.TotalSurcharges != 0 {
		lines = append(lines, kvColon("RECARGOS", money(rep.TotalSurcharges)))
	}
	lines = append(lines, rule("="))
	return lines
}

// RenderXReport renders the read-only X report (totals are NOT reset).
func RenderXReport(rep *state.Report, company state.Company) []string {
	lines := reportCommonLines(rep, company, "REPORTE X - CORTE DE CAJA")
	lines = append(lines, reportBodyLines(rep)...)
	lines = append(lines, center("ESTE REPORTE NO REINICIA"))
	lines = append(lines, center("LOS TOTALES DEL DIA"))
	lines = append(lines, rule("-"))
	if rep.LastInvoice != 0 {
		lines = append(lines, kvColon("ULTIMA FACTURA", fmt.Sprintf("%08d", rep.LastInvoice)))
	}
	lines = append(lines, center("* * * * * * * * * * * * * * * *"))
	lines = append(lines, center("SIMULADOR - NO VALOR FISCAL"))
	return lines
}

// RenderZReport renders the daily closure report (totals ARE reset after).
func RenderZReport(rep *state.Report, company state.Company) []string {
	lines := reportCommonLines(rep, company, "REPORTE Z - CIERRE DIARIO")
	lines = append(lines, reportBodyLines(rep)...)
	lines = append(lines, center("CIERRE DEL DIA"))
	lines = append(lines, center("TOTALES REINICIADOS TRAS"))
	lines = append(lines, center("ESTE REPORTE"))
	lines = append(lines, rule("-"))
	if rep.LastInvoice != 0 {
		lines = append(lines, kvColon("ULTIMA FACTURA", fmt.Sprintf("%08d", rep.LastInvoice)))
	}
	lines = append(lines, kvColon("CIERRES DIARIOS", fmt.Sprint(rep.Number)))
	lines = append(lines, center("* * * * * * * * * * * * * * * *"))
	lines = append(lines, center("SIMULADOR - NO VALOR FISCAL"))
	return lines
}

// RenderOpenDocument renders a live preview of the still-open document.
func RenderOpenDocument(s *state.State) []string {
	if !s.DocumentOpen {
		return nil
	}
	nextNumber := s.InvoiceCounter + 1
	switch s.DocumentType {
	case "credit_note":
		nextNumber = s.CreditNoteCounter + 1
	case "debit_note":
		nextNumber = s.DebitNoteCounter + 1
	case "non_fiscal":
		nextNumber = s.NonFiscalCounter + 1
	}
	docType := s.DocumentType
	if docType == "" {
		docType = "invoice"
	}
	doc := &state.Document{
		Type:           docType,
		Number:         nextNumber,
		Items:          append([]state.Item{}, s.CurrentItems...),
		SubtotalBases:  s.SubtotalBases,
		SubtotalTax:    s.SubtotalTax,
		Total:          s.SubtotalBases + s.SubtotalTax,
		Customer:       state.Customer{Rif: s.CustomerRif, Name: s.CustomerName, Address: s.CustomerAddress},
		Payments:       s.DocPayments,
		Paid:           s.PaymentsMade,
		Change:         maxf(0, s.PaymentsMade-(s.SubtotalBases+s.SubtotalTax)),
		Timestamp:      time.Now(),
		SerialNumber:   s.SerialNumber,
		Rif:            s.Rif,
		Company:        s.Company,
	}
	lines := RenderReceipt(doc, s.Company)
	out := []string{strings.Repeat(" ", PAPER_WIDTH)}
	out = append(out, pyfmt.Center("* DOCUMENTO EN CURSO - VISTA PREVIA *", PAPER_WIDTH))
	return append(out, lines...)
}

// RenderNonFiscal renders a closed non-fiscal document.
func RenderNonFiscal(doc *state.Document, company state.Company) []string {
	var lines []string
	lines = append(lines, companyBlock(company)...)
	lines = append(lines, rule("="))
	lines = append(lines, center("DOCUMENTO NO FISCAL"))
	lines = append(lines, center(fmt.Sprintf("N° %08d", doc.Number)))
	if !doc.Timestamp.IsZero() {
		lines = append(lines, center(ts(doc.Timestamp)))
	}
	lines = append(lines, rule("="))
	for _, item := range doc.Items {
		lines = append(lines, wrap(item.Description, 42)...)
	}
	lines = append(lines, rule("-"))
	lines = append(lines, center("DOCUMENTO SIN VALOR FISCAL"))
	return lines
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// Ensure orderedmap is referenced even if future refactors drop its use.
var _ = orderedmap.New[float64]
