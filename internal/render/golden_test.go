package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"simuladorfiscalgo/internal/orderedmap"
	"simuladorfiscalgo/internal/state"
)

func goldenPath(name string) string {
	return filepath.Join("..", "..", "testdata", "golden", name)
}

func readGolden(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(goldenPath(name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func joinLines(lines []string) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l)
		b.WriteString("\n")
	}
	return b.String()
}

func compareGolden(t *testing.T, name string, got []string) {
	t.Helper()
	want := readGolden(t, name)
	gotStr := joinLines(got)
	if gotStr == want {
		return
	}
	gw := strings.Split(strings.TrimRight(gotStr, "\n"), "\n")
	ww := strings.Split(strings.TrimRight(want, "\n"), "\n")
	for i := 0; i < len(gw) || i < len(ww); i++ {
		var g, w string
		if i < len(gw) {
			g = gw[i]
		}
		if i < len(ww) {
			w = ww[i]
		}
		if g != w {
			t.Errorf("%s line %d:\n got  %q\n want %q", name, i+1, g, w)
		}
	}
}

var fixedTS = time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

var testCompany = state.Company{
	Rif:                 "J-31055823-7",
	RazonSocial:         "DISTRIBUIDORA ANDELINA, C.A.",
	Direccion:           "AV. PRINCIPAL DE LAS MERCEDES, EDIF. TORRE AZUL, PISO 4, OFIC. 401",
	Municipio:           "CHACAO",
	Ciudad:              "CARACAS",
	Estado:              "DISTRITO CAPITAL",
	Telefono:            "0212-9512204",
	Email:               "ventas@andelina.com.ve",
	RepresentanteLegal:  "MARIA ELENA RODRIGUEZ SUAREZ",
	CedulaRepresentante: "V-10234567",
	ActividadEconomica:  "COMERCIO AL POR MENOR DE PRODUCTOS ALIMENTICIOS Y BEBIDAS",
	Moneda:              "Bs.",
	CondicionesPago:     "CONTADO / CRÉDITO",
	ContribuyenteEspecial: true,
	AgenteRetencion:     true,
}

// item mirrors Python's item() helper with tax included in the price.
func item(desc string, qty, price float64, taxType string, rate float64) state.Item {
	base := price / (1 + rate/100)
	tax := price - base
	return state.Item{
		Description: desc, Quantity: qty, Price: price,
		TaxType: taxType, TaxRate: rate,
		Base: base * qty, Tax: tax * qty,
		Total: (base + tax) * qty,
	}
}

func baseDoc() state.Document {
	items := []state.Item{
		item("ARROZ VENEZOLANO 1KG", 6, 2.5, "general", 16.0),
		item("ACEITE VEGETAL 1L", 3, 4.9, "general", 16.0),
		item("LECHE EN POLVO 1KG", 2, 7.5, "reduced", 8.0),
		item("PAN CASERO", 4, 1.2, "exempt", 0.0),
	}
	var sb, st float64
	for _, i := range items {
		sb += i.Base
		st += i.Tax
	}
	pay := orderedmap.New[float64]()
	pay.Set("cash", 66.35)
	return state.Document{
		Type:           "invoice",
		Number:         2,
		Items:          items,
		SubtotalBases:  sb,
		SubtotalTax:    st,
		Total:          sb + st,
		Customer: state.Customer{
			Rif: "J-50123456-1", Name: "COMERCIAL EL COMUNERO, C.A.",
			Address: "CALLE REAL DE CHACAO, NRO 15-23",
		},
		Payments:     pay,
		Paid:         66.35,
		Change:       maxf64(0, 66.35-(sb+st)),
		Timestamp:    fixedTS,
		SerialNumber: "MLTFHKA001",
		Rif:          "J-31055823-7",
		Company:      testCompany,
	}
}

func maxf64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func TestFixedReceipt(t *testing.T) {
	doc := baseDoc()
	compareGolden(t, "paper_fixed_receipt.txt", RenderReceipt(&doc, testCompany))
}

func TestFixedCreditNote(t *testing.T) {
	doc := baseDoc()
	doc.Type = "credit_note"
	doc.Number = 3
	doc.ReferenceInvoice = "00000001"
	doc.ReferenceDate = "2024-01-15"
	doc.ReferenceSerial = "MLTFHKA001"
	compareGolden(t, "paper_fixed_credit.txt", RenderReceipt(&doc, testCompany))
}

func TestFixedDebitNote(t *testing.T) {
	doc := baseDoc()
	doc.Type = "debit_note"
	doc.Number = 1
	doc.ReferenceInvoice = "00000001"
	doc.ReferenceDate = "2024-01-15"
	doc.ReferenceSerial = "MLTFHKA001"
	compareGolden(t, "paper_fixed_debit.txt", RenderReceipt(&doc, testCompany))
}

func TestFixedConsumer(t *testing.T) {
	pay := orderedmap.New[float64]()
	pay.Set("card", 9.02)
	doc := state.Document{
		Type: "invoice", Number: 1,
		Items:          []state.Item{item("CAFE MOLIDO 500G", 2, 3.9, "general", 16.0)},
		SubtotalBases:  6.724137931034483,
		SubtotalTax:    1.0758620689655173,
		Total:          7.8,
		Customer:       state.Customer{},
		Payments:       pay,
		Paid:           9.02,
		Change:         1.22,
		Timestamp:      fixedTS,
		SerialNumber:   "MLTFHKA001",
		Rif:            "J-31055823-7",
		Company:        testCompany,
	}
	compareGolden(t, "paper_fixed_consumer.txt", RenderReceipt(&doc, testCompany))
}

func TestFixedNonFiscal(t *testing.T) {
	doc := state.Document{
		Type: "non_fiscal", Number: 4,
		Items:     []state.Item{{Description: "PRESUPUESTO DE OBRA NRO 45"}},
		Timestamp: fixedTS,
		Company:   testCompany,
	}
	compareGolden(t, "paper_fixed_nonfiscal.txt", RenderNonFiscal(&doc, testCompany))
}

func baseReport() state.Report {
	pay := orderedmap.New[float64]()
	pay.Set("cash", 75.37)
	pay.Set("card", 9.02)
	return state.Report{
		Type: "X", Number: 1, Timestamp: fixedTS,
		LastInvoice: 2,
		TotalSales:  52.5, TotalTax: 6.278413793103448,
		DailyInvoices: 2,
		TaxTotals: map[string]state.TaxBlock{
			"general":    {Base: 32.33103448275862, Tax: 5.172965517241379},
			"reduced":    {Base: 13.889655172413794, Tax: 1.1111724137931034},
			"additional": {Base: 0, Tax: 0},
			"exempt":     {Base: 4.8, Tax: 0},
		},
		Payments: pay,
	}
}

func TestFixedXReport(t *testing.T) {
	rep := baseReport()
	compareGolden(t, "paper_fixed_xreport.txt", RenderXReport(&rep, testCompany))
}

func TestFixedZReport(t *testing.T) {
	rep := baseReport()
	rep.Type = "Z"
	compareGolden(t, "paper_fixed_zreport.txt", RenderZReport(&rep, testCompany))
}

func TestFixedEmptyXReport(t *testing.T) {
	rep := baseReport()
	rep.TotalSales = 0
	rep.TotalTax = 0
	rep.DailyInvoices = 0
	rep.LastInvoice = 0
	rep.TaxTotals = map[string]state.TaxBlock{
		"general": {}, "reduced": {}, "additional": {}, "exempt": {},
	}
	rep.Payments = orderedmap.New[float64]()
	compareGolden(t, "paper_fixed_empty_x.txt", RenderXReport(&rep, testCompany))
}

func TestFixedBareCompany(t *testing.T) {
	doc := baseDoc()
	doc.Company = state.Company{}
	empty := state.Company{}
	compareGolden(t, "paper_fixed_bare_company.txt", RenderReceipt(&doc, empty))
}

// TestWordsGolden checks number_to_words against the Python corpus.
func TestWordsGolden(t *testing.T) {
	data := readGolden(t, "words.txt")
	for _, line := range strings.Split(strings.TrimRight(data, "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			t.Fatalf("bad line %q", line)
		}
		var amount float64
		if _, err := fmtSscan(parts[0], &amount); err != nil {
			t.Fatalf("bad amount %q: %v", parts[0], err)
		}
		if got := NumberToWords(amount); got != parts[1] {
			t.Errorf("NumberToWords(%s) = %q, want %q", parts[0], got, parts[1])
		}
	}
}
