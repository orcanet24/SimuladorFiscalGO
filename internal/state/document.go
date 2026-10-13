package state

import (
	"time"

	"simuladorfiscalgo/internal/orderedmap"
	"simuladorfiscalgo/internal/pyfmt"
)

// CalculateItemTax splits a price into base and tax for the configured mode.
// taxType 1 means the tax is included in the price (Venezuela's default).
func (s *State) CalculateItemTax(price, taxRate float64) (base, tax float64) {
	if s.TaxType == 1 {
		base = price / (1 + taxRate/100)
		tax = price - base
	} else {
		base = price
		tax = price * taxRate / 100
	}
	return base, tax
}

// AddItem appends a line to the open document and updates every accumulator.
func (s *State) AddItem(description string, quantity, price float64, taxType string) Item {
	// Python: tax_rates.get(tax_type, 16.0), then exempt forces 0.
	taxRate, ok := s.TaxRates[taxType]
	if !ok {
		taxRate = 16.0
	}
	if taxType == "exempt" {
		taxRate = 0.0
	}
	base, tax := s.CalculateItemTax(price, taxRate)

	item := Item{
		Description: description,
		Quantity:    quantity,
		Price:       price,
		TaxType:     taxType,
		TaxRate:     taxRate,
		Base:        base * quantity,
		Tax:         tax * quantity,
		Total:       (base + tax) * quantity,
	}

	s.CurrentItems = append(s.CurrentItems, item)
	s.SubtotalBases += item.Base
	s.SubtotalTax += item.Tax
	s.AmountPayable += item.Total

	block := s.TaxTotals[taxType]
	block.Base += item.Base
	block.Tax += item.Tax
	s.TaxTotals[taxType] = block

	s.TotalSales += item.Total
	s.TotalTax += item.Tax
	s.DailySales += item.Total
	s.DailyTax += item.Tax

	return item
}

// AddPayment records a payment against the open document.
func (s *State) AddPayment(amount float64, method string) {
	if method == "" {
		method = "cash"
	}
	s.PaymentsMade += amount
	s.PaymentCount++
	s.AmountPayable -= amount

	s.PaymentTotals.Set(method, s.PaymentTotals.GetOr(method, 0)+amount)
	s.DocPayments.Set(method, s.DocPayments.GetOr(method, 0)+amount)
}

// OpenDocument starts a new document of the given type.
func (s *State) OpenDocument(docType string) {
	s.DocumentOpen = true
	s.DocumentType = docType
	s.CurrentItems = []Item{}
	s.SubtotalBases = 0
	s.SubtotalTax = 0
	s.AmountPayable = 0
	s.PaymentsMade = 0
	s.PaymentCount = 0
	s.DocPayments = orderedmap.New[float64]()
	s.SubtotalPrinted = false
	if docType == "non_fiscal" {
		s.CurrentState = "non_fiscal"
	} else {
		s.CurrentState = "fiscal"
	}
}

// CloseDocument freezes the open document into a snapshot and resets every
// document-scoped accumulator. Returns nil when nothing is open.
func (s *State) CloseDocument() *Document {
	if !s.DocumentOpen {
		return nil
	}

	doc := &Document{
		Type:           s.DocumentType,
		Items:          append([]Item{}, s.CurrentItems...),
		SubtotalBases:  s.SubtotalBases,
		SubtotalTax:    s.SubtotalTax,
		Total:          s.SubtotalBases + s.SubtotalTax,
		Customer:       Customer{Rif: s.CustomerRif, Name: s.CustomerName, Address: s.CustomerAddress},
		Payments:       s.DocPayments.Clone(),
		Paid:           s.PaymentsMade,
		Change:         maxf(0, s.PaymentsMade-(s.SubtotalBases+s.SubtotalTax)),
		Timestamp:      time.Now(),
		SerialNumber:   s.SerialNumber,
		Rif:            s.Rif,
		Company:        s.Company,
		ReferenceDate:  s.ReferenceDate,
		ReferenceSerial: s.ReferenceSerial,
	}

	switch s.DocumentType {
	case "invoice":
		s.InvoiceCounter++
		s.FiscalCounter++
		s.DailyInvoices++
		doc.Number = s.InvoiceCounter
		now := time.Now().Format(time.RFC3339)
		s.LastInvoiceDate = &now
	case "credit_note":
		s.CreditNoteCounter++
		s.FiscalCounter++
		s.DailyCreditNotes++
		doc.Number = s.CreditNoteCounter
		doc.ReferenceInvoice = s.ReferenceInvoice
	case "debit_note":
		s.DebitNoteCounter++
		s.FiscalCounter++
		s.DailyDebitNotes++
		doc.Number = s.DebitNoteCounter
		doc.ReferenceInvoice = s.ReferenceInvoice
	case "non_fiscal":
		s.NonFiscalCounter++
		s.DailyNonFiscal++
		doc.Number = s.NonFiscalCounter
	}

	s.DocumentOpen = false
	s.DocumentType = ""
	s.CurrentState = "waiting"
	s.CurrentItems = []Item{}
	s.SubtotalBases = 0
	s.SubtotalTax = 0
	s.AmountPayable = 0
	s.PaymentsMade = 0
	s.PaymentCount = 0
	s.DocPayments = orderedmap.New[float64]()
	s.SubtotalPrinted = false
	// Customer/reference data does not carry over to the next document.
	s.CustomerRif = ""
	s.CustomerName = ""
	s.CustomerAddress = ""
	s.CustomerPhone = ""
	s.ReferenceInvoice = ""
	s.ReferenceDate = ""
	s.ReferenceSerial = ""
	s.PendingPaymentMethod = ""

	// Approximate the audit-memory cost of the document (KB -> MB).
	sizeKB := float64(len(docString(doc)) / 1024)
	s.AuditMemoryUsed += sizeKB / 1024
	s.AuditMemoryFree = s.AuditMemoryTotal - s.AuditMemoryUsed

	return doc
}

// docString is the Python len(str(doc)) approximation used for memory usage.
func docString(doc *Document) string {
	n := len(doc.Items)
	// Python's str(dict) length scales with the number of entries; the exact
	// value only affects the audit-memory percentage shown on the panel.
	return string(make([]byte, 900+n*180+len(doc.Customer.Name)))
}

// GenerateZReport snapshots the day and resets the daily accumulators.
func (s *State) GenerateZReport() *Report {
	rep := &Report{
		Type:              "Z",
		Number:            s.ZReportCounter + 1,
		Timestamp:         time.Now(),
		LastInvoice:       s.InvoiceCounter,
		LastCreditNote:    s.CreditNoteCounter,
		LastDebitNote:     s.DebitNoteCounter,
		TotalSales:        s.DailySales,
		TotalTax:          s.DailyTax,
		TotalDiscounts:    s.DailyDiscounts,
		TotalSurcharges:   s.DailySurcharges,
		DailyInvoices:     s.DailyInvoices,
		DailyCreditNotes:  s.DailyCreditNotes,
		DailyDebitNotes:   s.DailyDebitNotes,
		DailyNonFiscal:    s.DailyNonFiscal,
		LifetimeSales:     s.TotalSales,
		LifetimeTax:       s.TotalTax,
		TaxTotals:         copyBlockMap(s.TaxTotals),
		CreditNoteTotals:  copyBlockMap(s.CreditNoteTotals),
		DebitNoteTotals:   copyBlockMap(s.DebitNoteTotals),
		Payments:          s.PaymentTotals.Clone(),
	}

	s.ZReportCounter++
	s.DailyClosureCounter++
	now := time.Now().Format(time.RFC3339)
	s.LastZReportDate = &now
	s.ZReports = append(s.ZReports, rep)

	// Reset only the daily totals; lifetime counters are never reset.
	s.DailySales = 0
	s.DailyTax = 0
	s.DailyDiscounts = 0
	s.DailySurcharges = 0
	s.DailyInvoices = 0
	s.DailyCreditNotes = 0
	s.DailyDebitNotes = 0
	s.DailyNonFiscal = 0
	s.TaxTotals = newTaxBlockMap()
	s.CreditNoteTotals = newTaxBlockMap()
	s.DebitNoteTotals = newTaxBlockMap()
	s.PaymentTotals = orderedmap.New[float64]()

	return rep
}

// GenerateXReport snapshots the day without resetting anything.
func (s *State) GenerateXReport() *Report {
	rep := &Report{
		Type:             "X",
		Number:           s.XReportCounter + 1,
		Timestamp:        time.Now(),
		LastInvoice:      s.InvoiceCounter,
		LastCreditNote:   s.CreditNoteCounter,
		LastDebitNote:    s.DebitNoteCounter,
		TotalSales:       s.DailySales,
		TotalTax:         s.DailyTax,
		DailyInvoices:    s.DailyInvoices,
		DailyCreditNotes: s.DailyCreditNotes,
		DailyDebitNotes:  s.DailyDebitNotes,
		DailyNonFiscal:   s.DailyNonFiscal,
		TaxTotals:        copyBlockMap(s.TaxTotals),
		Payments:         s.PaymentTotals.Clone(),
	}
	s.XReportCounter++
	s.XReports = append(s.XReports, rep)
	return rep
}

func copyBlockMap(m map[string]TaxBlock) map[string]TaxBlock {
	out := make(map[string]TaxBlock, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// S1Data is the TFHKA S1 general-parameters payload.
type S1Data struct {
	MachineNumber     string
	Rif               string
	AuditCounter      int
	DailyClosure      int
	LastInvoice       int
	LastCreditNote    int
	LastDebitNote     int
	LastNonFiscal     int
	InvoicesToday     int
	CreditNotesToday  int
	DebitNotesToday   int
	NonFiscalToday    int
	TotalSales        float64
	CurrentDatetime   string
	CashierNumber     int
}

// GetS1Data builds the S1 response fields.
func (s *State) GetS1Data() S1Data {
	return S1Data{
		MachineNumber:    s.SerialNumber,
		Rif:              s.Rif,
		AuditCounter:     s.FiscalCounter,
		DailyClosure:     s.DailyClosureCounter,
		LastInvoice:      s.InvoiceCounter,
		LastCreditNote:   s.CreditNoteCounter,
		LastDebitNote:    s.DebitNoteCounter,
		LastNonFiscal:    s.NonFiscalCounter,
		InvoicesToday:    s.DailyInvoices,
		CreditNotesToday: s.DailyCreditNotes,
		DebitNotesToday:  s.DailyDebitNotes,
		NonFiscalToday:   s.DailyNonFiscal,
		TotalSales:       s.DailySales,
		CurrentDatetime:  time.Now().Format("2006-01-02 15:04:05"),
		CashierNumber:    1,
	}
}

// S2Data is the TFHKA S2 document-status payload.
type S2Data struct {
	Condition      int
	TypeDocument   int
	QuantityArticles int
	SubtotalBases  float64
	SubtotalTax    float64
	AmountPayable  float64
	PaymentsMade   int
}

// GetS2Data builds the S2 response fields.
func (s *State) GetS2Data() S2Data {
	cond := 1
	if s.DocumentOpen {
		cond = 0
	}
	docType := 0
	switch s.DocumentType {
	case "invoice":
		docType = 0
	case "credit_note":
		docType = 1
	case "debit_note":
		docType = 2
	}
	return S2Data{
		Condition:        cond,
		TypeDocument:     docType,
		QuantityArticles: len(s.CurrentItems),
		SubtotalBases:    round2(s.SubtotalBases),
		SubtotalTax:      round2(s.SubtotalTax),
		AmountPayable:    round2(s.AmountPayable),
		PaymentsMade:     s.PaymentCount,
	}
}

// S3Data is the TFHKA S3 tax-configuration payload.
type S3Data struct {
	Tax1     float64
	Tax2     float64
	Tax3     float64
	TypeTax1 int
	TypeTax2 int
	TypeTax3 int
}

// GetS3Data builds the S3 response fields.
func (s *State) GetS3Data() S3Data {
	return S3Data{
		Tax1:     s.TaxRates["general"],
		Tax2:     s.TaxRates["reduced"],
		Tax3:     s.TaxRates["additional"],
		TypeTax1: s.TaxType,
		TypeTax2: s.TaxType,
		TypeTax3: s.TaxType,
	}
}

// S5Data is the TFHKA S5 audit-memory payload.
type S5Data struct {
	MachineNumber      string
	Rif                string
	AuditMemoryNumber  int
	AuditMemoryTotal   float64
	AuditMemoryFree    float64
	RegisteredDocuments int
}

// GetS5Data builds the S5 response fields.
func (s *State) GetS5Data() S5Data {
	return S5Data{
		MachineNumber:       s.SerialNumber,
		Rif:                 s.Rif,
		AuditMemoryNumber:   s.AuditMemoryNumber,
		AuditMemoryTotal:    s.AuditMemoryTotal,
		AuditMemoryFree:     round2(s.AuditMemoryFree),
		RegisteredDocuments: s.FiscalCounter,
	}
}

func round2(v float64) float64 {
	return pyfmt.PyRound(v, 2)
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
