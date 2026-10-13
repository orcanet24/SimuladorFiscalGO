// Package state models the internal state of a fiscal printer: counters,
// the open document, fiscal memory (company data) and the paper job queue.
package state

import (
	"strings"
	"sync"
	"time"

	"simuladorfiscalgo/internal/orderedmap"
	"simuladorfiscalgo/internal/pyfmt"
)

// TaxBlock accumulates base and tax for one tax rate.
type TaxBlock struct {
	Base float64 `json:"base"`
	Tax  float64 `json:"tax"`
}

// Item is a single line of the open document.
type Item struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	Price       float64 `json:"price"`
	TaxType     string  `json:"tax_type"`
	TaxRate     float64 `json:"tax_rate"`
	Base        float64 `json:"base"`
	Tax         float64 `json:"tax"`
	Total       float64 `json:"total"`
}

// Customer holds the customer block of a document.
type Customer struct {
	Rif     string `json:"rif"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

// Company is the fiscal memory: the data the law requires on every receipt.
type Company struct {
	Rif                 string `json:"rif"`
	RazonSocial         string `json:"razon_social"`
	Direccion           string `json:"direccion"`
	Municipio           string `json:"municipio"`
	Ciudad              string `json:"ciudad"`
	Estado              string `json:"estado"`
	Telefono            string `json:"telefono"`
	Email               string `json:"email"`
	RepresentanteLegal  string `json:"representante_legal"`
	CedulaRepresentante string `json:"cedula_representante"`
	ActividadEconomica  string `json:"actividad_economica"`
	NIF                 string `json:"nif"`
	Moneda              string `json:"moneda"`
	LogoText            string `json:"logo_text"`
	CondicionesPago     string `json:"condiciones_pago"`
	ContribuyenteEspecial bool  `json:"contribuyente_especial"`
	AgenteRetencion     bool  `json:"agente_retencion"`
}

// Set assigns a company field from the API's generic dictionary, ignoring
// unknown keys exactly like FiscalPrinterState.program_company does.
func (c *Company) Set(key string, value any) {
	s, _ := value.(string)
	b, _ := value.(bool)
	switch key {
	case "rif":
		c.Rif = s
	case "razon_social":
		c.RazonSocial = s
	case "direccion":
		c.Direccion = s
	case "municipio":
		c.Municipio = s
	case "ciudad":
		c.Ciudad = s
	case "estado":
		c.Estado = s
	case "telefono":
		c.Telefono = s
	case "email":
		c.Email = s
	case "representante_legal":
		c.RepresentanteLegal = s
	case "cedula_representante":
		c.CedulaRepresentante = s
	case "actividad_economica":
		c.ActividadEconomica = s
	case "nif":
		c.NIF = s
	case "moneda":
		c.Moneda = s
	case "logo_text":
		c.LogoText = s
	case "condiciones_pago":
		c.CondicionesPago = s
	case "contribuyente_especial":
		c.ContribuyenteEspecial = b
	case "agente_retencion":
		c.AgenteRetencion = b
	}
}

// NewCompany returns a company with the reference defaults.
func NewCompany() Company {
	return Company{
		Moneda:          "Bs.",
		CondicionesPago: "CONTADO",
	}
}

// HistoryEntry is one row of the command log shown on the panel.
type HistoryEntry struct {
	Seq      int    `json:"seq"`
	TS       string `json:"ts"`
	Command  string `json:"command"`
	Response string `json:"response"`
	OK       bool   `json:"ok"`
}

// PrintJob is a finished paper output (receipt or report).
type PrintJob struct {
	ID    int      `json:"id"`
	Kind  string   `json:"kind"`
	Title string   `json:"title"`
	TS    string   `json:"ts"`
	Brand string   `json:"brand"`
	Lines []string `json:"lines"`
	Meta  *JobMeta `json:"meta"`
}

// Report is an X or Z report snapshot.
type Report struct {
	Type              string                      `json:"type"`
	Number            int                         `json:"number"`
	Timestamp         time.Time                   `json:"timestamp"`
	LastInvoice       int                         `json:"last_invoice"`
	LastCreditNote    int                         `json:"last_credit_note"`
	LastDebitNote     int                         `json:"last_debit_note"`
	TotalSales        float64                     `json:"total_sales"`
	TotalTax          float64                     `json:"total_tax"`
	TotalDiscounts    float64                     `json:"total_discounts,omitempty"`
	TotalSurcharges   float64                     `json:"total_surcharges,omitempty"`
	DailyInvoices     int                         `json:"daily_invoices"`
	DailyCreditNotes  int                         `json:"daily_credit_notes"`
	DailyDebitNotes   int                         `json:"daily_debit_notes"`
	DailyNonFiscal    int                         `json:"daily_non_fiscal"`
	LifetimeSales     float64                     `json:"lifetime_sales,omitempty"`
	LifetimeTax       float64                     `json:"lifetime_tax,omitempty"`
	TaxTotals         map[string]TaxBlock         `json:"tax_totals"`
	CreditNoteTotals  map[string]TaxBlock         `json:"credit_note_totals,omitempty"`
	DebitNoteTotals   map[string]TaxBlock         `json:"debit_note_totals,omitempty"`
	Payments          *orderedmap.Map[float64]    `json:"payments"`
}

// Document is a closed document snapshot handed to the paper renderer.
type Document struct {
	Type             string                    `json:"type"`
	Number           int                       `json:"number"`
	Items            []Item                    `json:"items"`
	SubtotalBases    float64                   `json:"subtotal_bases"`
	SubtotalTax      float64                   `json:"subtotal_tax"`
	Total            float64                   `json:"total"`
	Customer         Customer                  `json:"customer"`
	Payments         *orderedmap.Map[float64]  `json:"payments"`
	Paid             float64                   `json:"paid"`
	Change           float64                   `json:"change"`
	Timestamp        time.Time                 `json:"timestamp"`
	SerialNumber     string                    `json:"serial_number"`
	Rif              string                    `json:"rif"`
	Company          Company                   `json:"company"`
	ReferenceInvoice string                    `json:"reference_invoice,omitempty"`
	ReferenceDate    string                    `json:"reference_date,omitempty"`
	ReferenceSerial  string                    `json:"reference_serial,omitempty"`
}

// State is the full internal state of one simulated printer.
type State struct {
	SerialNumber       string  `json:"serial_number"`
	FiscalRegistration string  `json:"fiscal_registration"`
	Rif                string  `json:"rif"`
	Mode               string  `json:"mode"`
	MemoryStatus       string  `json:"memory_status"`
	CurrentState       string  `json:"current_state"`

	FiscalCounter       int `json:"fiscal_counter"`
	InvoiceCounter      int `json:"invoice_counter"`
	CreditNoteCounter   int `json:"credit_note_counter"`
	DebitNoteCounter    int `json:"debit_note_counter"`
	NonFiscalCounter    int `json:"non_fiscal_counter"`
	ZReportCounter      int `json:"z_report_counter"`
	XReportCounter      int `json:"x_report_counter"`
	DailyClosureCounter int `json:"daily_closure_counter"`

	TotalSales      float64 `json:"total_sales"`
	TotalTax        float64 `json:"total_tax"`
	TotalDiscounts  float64 `json:"total_discounts"`
	TotalSurcharges float64 `json:"total_surcharges"`

	DailySales         float64 `json:"daily_sales"`
	DailyTax           float64 `json:"daily_tax"`
	DailyDiscounts     float64 `json:"daily_discounts"`
	DailySurcharges    float64 `json:"daily_surcharges"`
	DailyInvoices      int     `json:"daily_invoices"`
	DailyCreditNotes   int     `json:"daily_credit_notes"`
	DailyDebitNotes    int     `json:"daily_debit_notes"`
	DailyNonFiscal     int     `json:"daily_non_fiscal"`

	TaxTotals        map[string]TaxBlock `json:"tax_totals"`
	CreditNoteTotals map[string]TaxBlock `json:"credit_note_totals"`
	DebitNoteTotals  map[string]TaxBlock `json:"debit_note_totals"`

	PaymentTotals *orderedmap.Map[float64] `json:"payment_totals"`

	Company  Company         `json:"company"`
	Headers  []string        `json:"headers"`
	Footers  map[string]string `json:"footers"`
	Flags    map[string]bool `json:"flags"`

	DocumentOpen   bool    `json:"document_open"`
	DocumentType   string  `json:"document_type"`
	CurrentItems   []Item  `json:"current_items"`
	SubtotalBases  float64 `json:"subtotal_bases"`
	SubtotalTax    float64 `json:"subtotal_tax"`
	AmountPayable  float64 `json:"amount_payable"`
	PaymentsMade   float64 `json:"payments_made"`
	PaymentCount   int     `json:"payment_count"`
	DocPayments    *orderedmap.Map[float64] `json:"doc_payments"`
	SubtotalPrinted bool   `json:"subtotal_printed"`

	CustomerRif      string `json:"customer_rif"`
	CustomerName     string `json:"customer_name"`
	CustomerAddress  string `json:"customer_address"`
	CustomerPhone    string `json:"customer_phone"`
	ReferenceInvoice string `json:"reference_invoice"`
	ReferenceDate    string `json:"reference_date"`
	ReferenceSerial  string `json:"reference_serial"`

	AuditMemoryNumber int     `json:"audit_memory_number"`
	AuditMemoryTotal  float64 `json:"audit_memory_total"`
	AuditMemoryFree   float64 `json:"audit_memory_free"`
	AuditMemoryUsed   float64 `json:"audit_memory_used"`

	ZReports   []*Report  `json:"z_reports"`
	XReports   []*Report  `json:"x_reports"`
	PrintJobs  []*PrintJob `json:"print_jobs"`
	PrintJobSeq int       `json:"print_job_seq"`

	TaxRates map[string]float64 `json:"tax_rates"`
	TaxType  int                `json:"tax_type"`

	LastInvoiceDate *string `json:"last_invoice_date"`
	LastZReportDate *string `json:"last_z_report_date"`

	// Runtime-only fields, never persisted.
	PendingPaymentMethod string `json:"-"`

	Brand string `json:"-"`
	Model string `json:"-"`

	CustomerNameLine2 string `json:"-"`

	HasPaper  bool `json:"-"`
	PaperJam  bool `json:"-"`
	CoverOpen bool `json:"-"`

	Mu sync.Mutex `json:"-"`
}

// NewState builds the default state for a brand/model pair.
func NewState(brand, model string) *State {
	brand = strings.ToUpper(brand)
	model = strings.ToUpper(model)
	serial := "ML" + take(brand, 2) + take(model, 3) + "10000"

	s := &State{
		SerialNumber:       serial,
		FiscalRegistration: "J-12345678-9",
		Rif:                "J-12345678-9",
		Mode:               "fiscal",
		MemoryStatus:       "ok",
		CurrentState:       "waiting",
		TaxTotals:          newTaxBlockMap(),
		CreditNoteTotals:   newTaxBlockMap(),
		DebitNoteTotals:    newTaxBlockMap(),
		PaymentTotals:      orderedmap.New[float64](),
		DocPayments:        orderedmap.New[float64](),
		Company:            NewCompany(),
		Headers:            []string{"RIF: J-12345678-9", "FACTURA FISCAL", "AVENIDA PRINCIPAL", "CARACAS - VENEZUELA", "", "", "", ""},
		Footers:            map[string]string{"footer1": "GRACIAS POR SU COMPRA"},
		Flags: map[string]bool{
			"print_logo": false, "reduce_z_report": false,
			"print_z_report_items": false, "print_customer_data": false,
			"dont_open_drawer": false, "paper_sensor_enabled": true,
		},
		TaxRates: map[string]float64{
			"general": 16.0, "reduced": 8.0, "additional": 30.0,
		},
		TaxType:           1,
		AuditMemoryNumber: 1,
		AuditMemoryTotal:  4.0,
		AuditMemoryFree:   3.5,
		AuditMemoryUsed:   0.5,
		CurrentItems:      []Item{},
		PrintJobs:         []*PrintJob{},
		ZReports:          []*Report{},
		XReports:          []*Report{},
		HasPaper:          true,
		Brand:             brand,
		Model:             model,
	}
	return s
}

func take(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func newTaxBlockMap() map[string]TaxBlock {
	return map[string]TaxBlock{
		"general": {}, "reduced": {}, "additional": {}, "exempt": {},
	}
}

// ProgramCompany programs company data into the fiscal memory and
// regenerates the eight S8E header lines.
func (s *State) ProgramCompany(data map[string]any) Company {
	for k, v := range data {
		s.Company.Set(k, v)
	}
	if s.Company.Rif != "" {
		s.Rif = s.Company.Rif
		s.FiscalRegistration = s.Company.Rif
	}
	c := s.Company
	logo := c.LogoText
	if logo == "" {
		logo = c.RazonSocial
	}
	locality := joinNonEmpty([]string{c.Municipio, c.Estado}, " - ")
	headers := make([]string, 8)
	headers[0] = pyfmt.Slice(logo, 0, 40)
	headers[1] = pyfmt.Slice("RIF: "+c.Rif, 0, 40)
	headers[2] = pyfmt.Slice(c.Direccion, 0, 40)
	headers[3] = pyfmt.Slice(locality, 0, 40)
	if c.Telefono != "" {
		headers[4] = pyfmt.Slice("TEL: "+c.Telefono, 0, 40)
	}
	if c.RepresentanteLegal != "" {
		headers[5] = pyfmt.Slice("REP. LEGAL: "+c.RepresentanteLegal, 0, 40)
	}
	s.Headers = headers
	return c
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

// GetStatusCode returns the current TFHKA status byte.
func (s *State) GetStatusCode() byte {
	return statusByte(s.Mode, s.CurrentState, s.MemoryStatus)
}

func statusByte(mode, cur, memory string) byte {
	if mode == "fiscal" {
		switch cur {
		case "waiting":
			switch memory {
			case "ok":
				return 0x04
			case "almost_full":
				return 0x07
			default:
				return 0x0A
			}
		case "fiscal":
			switch memory {
			case "ok":
				return 0x05
			case "almost_full":
				return 0x08
			default:
				return 0x0B
			}
		case "non_fiscal":
			switch memory {
			case "ok":
				return 0x06
			case "almost_full":
				return 0x09
			default:
				return 0x0C
			}
		}
	}
	return 0x04
}

// GetErrorCode returns the current TFHKA error byte.
func (s *State) GetErrorCode() byte {
	if !s.HasPaper {
		return 0x01
	}
	if s.PaperJam {
		return 0x02
	}
	if s.AuditMemoryFree <= 0.01 {
		return 0x6C
	}
	return 0x00
}
