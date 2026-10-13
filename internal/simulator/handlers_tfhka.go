package simulator

import (
	"fmt"
	"strconv"
	"strings"

	"simuladorfiscalgo/internal/pyfmt"
	"simuladorfiscalgo/internal/state"
)

// handleTFHKA executes a parsed TFHKA ASCII command.
func (s *Simulator) handleTFHKA(cmdType string, params []any, raw string) (string, error) {
	st := s.State

	switch cmdType {
	case "ITEM", "ITEM_CREDIT", "ITEM_DEBIT":
		return s.handleItem(raw, cmdType, params)

	case "SUBTOTAL_PRINT":
		st.SubtotalPrinted = true
		return fmt.Sprintf("0|Subtotal: %s + %s = %s",
			pyfmt.F2(st.SubtotalBases), pyfmt.F2(st.SubtotalTax),
			pyfmt.F2(st.SubtotalBases+st.SubtotalTax)), nil

	case "SUBTOTAL_SILENT":
		return "0|OK", nil

	case "CLOSE_TOTALIZE":
		if st.DocumentOpen {
			doc := st.CloseDocument()
			if doc != nil {
				job := s.recordReceipt(doc)
				return fmt.Sprintf("0|%s %d|Total: %s|Impreso:%d",
					docTypeName(doc.Type), doc.Number, pyfmt.F2(doc.Total), job.ID), nil
			}
		}
		return "0|Cerrado", nil

	case "OPEN_DRAWER":
		return "0|Gaveta abierta", nil

	case "OPEN_NON_FISCAL":
		st.OpenDocument("non_fiscal")
		return "0|DNF abierto", nil

	case "PRINT_NON_FISCAL_TEXT":
		return "0|Texto impreso", nil

	case "PRINT_NON_FISCAL_CONTENT":
		return "0|Contenido impreso", nil

	case "CLOSE_NON_FISCAL":
		if st.DocumentOpen {
			if doc := st.CloseDocument(); doc != nil {
				s.recordReceipt(doc)
			}
		}
		return "0|DNF cerrado", nil

	case "COMMENT":
		return "0|Comentario", nil

	case "PAYMENT":
		amount, err := pyfmt.Float(strParam(params, 0))
		if err != nil {
			return "", err
		}
		// A preceding "103 <medio>" declares this payment's method.
		method := st.PendingPaymentMethod
		if method == "" {
			method = "cash"
		}
		st.PendingPaymentMethod = ""
		st.AddPayment(amount, method)
		return fmt.Sprintf("0|Pago: %s|Pendiente: %s",
			pyfmt.F2(amount), pyfmt.F2(st.AmountPayable)), nil

	case "PAYMENT_DESC":
		desc := strings.ToLower(strings.TrimSpace(strParam(params, 0)))
		if method, ok := paymentMethodMap[desc]; ok {
			st.PendingPaymentMethod = method
		}
		return "0|Descripcion pago", nil

	case "DISCOUNT":
		amount, err := pyfmt.Float(strParam(params, 0))
		if err != nil {
			return "", err
		}
		st.TotalDiscounts += amount
		st.AmountPayable -= amount
		return fmt.Sprintf("0|Descuento: %s", pyfmt.F2(amount)), nil

	case "SURCHARGE":
		amount, err := pyfmt.Float(strParam(params, 0))
		if err != nil {
			return "", err
		}
		st.TotalSurcharges += amount
		st.AmountPayable += amount
		return fmt.Sprintf("0|Recargo: %s", pyfmt.F2(amount)), nil

	case "CUSTOMER_RIF":
		st.CustomerRif = strings.ReplaceAll(strParam(params, 0), "*", "")
		return "0|RIF registrado", nil

	case "CUSTOMER_NAME":
		st.CustomerName = strings.ReplaceAll(strParam(params, 0), "*", "")
		return "0|Nombre registrado", nil

	case "CUSTOMER_ADDRESS":
		st.CustomerAddress = strParam(params, 0)
		return "0|Direccion registrada", nil

	case "CUSTOMER_PHONE":
		st.CustomerPhone = strParam(params, 0)
		return "0|Telefono registrado", nil

	case "INVOICE_NUMBER":
		st.ReferenceInvoice = strings.ReplaceAll(strParam(params, 0), "*", "")
		return "0|Nro factura referenciada", nil

	case "INVOICE_DATE":
		st.ReferenceDate = strings.ReplaceAll(strParam(params, 0), "*", "")
		return "0|Fecha factura referenciada", nil

	case "FISCAL_SERIAL":
		st.ReferenceSerial = strings.ReplaceAll(strParam(params, 0), "*", "")
		return "0|Serial fiscal registrado", nil

	case "OPEN_CREDIT_NOTE":
		st.OpenDocument("credit_note")
		return "0|Nota credito abierta", nil

	case "OPEN_DEBIT_NOTE":
		st.OpenDocument("debit_note")
		return "0|Nota debito abierta", nil

	case "S1":
		d := st.GetS1Data()
		return fmt.Sprintf("0|%s|%s|%d|%d|%d|%d|%d|%d|%s|%s",
			d.MachineNumber, d.Rif, d.AuditCounter, d.DailyClosure,
			d.LastInvoice, d.LastCreditNote, d.LastDebitNote, d.LastNonFiscal,
			pyfmt.F2(d.TotalSales), d.CurrentDatetime), nil

	case "S2":
		d := st.GetS2Data()
		return fmt.Sprintf("%d|%d|%d|%s|%s|%s|%d",
			d.Condition, d.TypeDocument, d.QuantityArticles,
			pyfmt.F2(d.SubtotalBases), pyfmt.F2(d.SubtotalTax),
			pyfmt.F2(d.AmountPayable), d.PaymentsMade), nil

	case "S3":
		d := st.GetS3Data()
		return fmt.Sprintf("%s|%s|%s|%d|%d|%d",
			pyfmt.F2(d.Tax1), pyfmt.F2(d.Tax2), pyfmt.F2(d.Tax3),
			d.TypeTax1, d.TypeTax2, d.TypeTax3), nil

	case "S4":
		var parts []string
		st.PaymentTotals.ForEach(func(_ string, v float64) {
			parts = append(parts, pyfmt.F2(v))
		})
		return "0|" + strings.Join(parts, "|") + "|0.00", nil

	case "S5":
		d := st.GetS5Data()
		return fmt.Sprintf("%s|%s|%d|%s|%s|%d",
			d.MachineNumber, d.Rif, d.AuditMemoryNumber,
			pyfmt.F2(d.AuditMemoryTotal), pyfmt.F2(d.AuditMemoryFree),
			d.RegisteredDocuments), nil

	case "S8E":
		return strings.Join(st.Headers, "|"), nil

	case "S8P":
		var footers []string
		for i := 1; i <= 8; i++ {
			footers = append(footers, st.Footers[fmt.Sprintf("footer%d", i)])
		}
		return strings.Join(footers, "|"), nil

	case "U0X", "REPORT_X":
		x := st.GenerateXReport()
		job := s.recordReport(x, "X")
		return fmt.Sprintf("0|Reporte X #%d|Ventas: %s|IVA: %s|Impreso:%d",
			x.Number, pyfmt.F2(x.TotalSales), pyfmt.F2(x.TotalTax), job.ID), nil

	case "U1X", "REPORT_X2":
		x := st.GenerateXReport()
		job := s.recordReport(x, "X")
		return fmt.Sprintf("0|Reporte X2 #%d|Ventas: %s|Impreso:%d",
			x.Number, pyfmt.F2(x.TotalSales), job.ID), nil

	case "U0Z", "REPORT_Z":
		z := st.GenerateZReport()
		job := s.recordReport(z, "Z")
		return fmt.Sprintf("0|Reporte Z #%d|Ventas: %s|IVA: %s|Impreso:%d",
			z.Number, pyfmt.F2(z.TotalSales), pyfmt.F2(z.TotalTax), job.ID), nil

	case "U1Z", "REPORT_Z2":
		z := st.GenerateZReport()
		job := s.recordReport(z, "Z")
		return fmt.Sprintf("0|Reporte Z2 #%d|Ventas: %s|Impreso:%d",
			z.Number, pyfmt.F2(z.TotalSales), job.ID), nil
	}

	return "ERROR|0x50|Comando no implementado", nil
}

func docTypeName(docType string) string {
	switch docType {
	case "invoice":
		return "Factura"
	case "credit_note":
		return "Nota de Credito"
	case "debit_note":
		return "Nota de Debito"
	case "non_fiscal":
		return "Doc No Fiscal"
	}
	return "Documento"
}

// strParam returns params[i] as a string, or "" when absent.
func strParam(params []any, i int) string {
	if i >= len(params) || params[i] == nil {
		return ""
	}
	switch v := params[i].(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case fmt.Stringer:
		return v.String()
	}
	return fmt.Sprintf("%v", params[i])
}

// handleItem parses the fixed-width TFHKA item command formats.
func (s *Simulator) handleItem(raw, cmdType string, params []any) (string, error) {
	st := s.State
	if !st.DocumentOpen {
		st.OpenDocument("invoice")
	}

	switch cmdType {
	case "ITEM":
		if len([]rune(raw)) < 20 {
			return "ERROR|0x50|Item format invalid", nil
		}
		taxType := map[byte]string{' ': "exempt", '!': "general", '"': "reduced", '#': "additional"}[raw[0]]
		if taxType == "" {
			taxType = "general"
		}
		qtyStr := strings.TrimSpace(field(raw, 1, 9))
		priceStr := strings.TrimSpace(field(raw, 9, 19))
		desc := strings.TrimSpace(field(raw, 19, len(raw)))
		qty, err := pyfmt.Float(qtyStr)
		if err != nil {
			return "ERROR|0x50|Invalid qty/price", nil
		}
		price, err := pyfmt.Float(priceStr)
		if err != nil {
			return "ERROR|0x50|Invalid qty/price", nil
		}
		if desc == "" {
			desc = "PRODUCTO"
		}
		item := st.AddItem(desc, qty, price, taxType)
		return fmt.Sprintf("0|Item|%s|%s|%s|%s|%s",
			desc, pyfmt.F3(qty), pyfmt.F2(price), pyfmt.F2(item.Tax), pyfmt.F2(item.Total)), nil

	case "ITEM_CREDIT":
		if len([]rune(raw)) < 21 {
			return "ERROR|0x50|Credit note item format invalid", nil
		}
		taxType := taxName(intParam(params, 0, 1))
		qty, price, desc, ok := parseNoteItem(raw, 2, "PRODUCTO NC")
		if !ok {
			return "ERROR|0x50|Invalid qty/price", nil
		}
		item := st.AddItem(desc, qty, price, taxType)
		b := st.CreditNoteTotals[taxType]
		b.Base += item.Base
		b.Tax += item.Tax
		st.CreditNoteTotals[taxType] = b
		return fmt.Sprintf("0|NC Item|%s|%s|%s|%s|%s",
			desc, pyfmt.F3(qty), pyfmt.F2(price), pyfmt.F2(item.Tax), pyfmt.F2(item.Total)), nil

	case "ITEM_DEBIT":
		if len([]rune(raw)) < 21 {
			return "ERROR|0x50|Debit note item format invalid", nil
		}
		taxType := taxName(intParam(params, 0, 1))
		qty, price, desc, ok := parseNoteItem(raw, 2, "PRODUCTO ND")
		if !ok {
			return "ERROR|0x50|Invalid qty/price", nil
		}
		item := st.AddItem(desc, qty, price, taxType)
		b := st.DebitNoteTotals[taxType]
		b.Base += item.Base
		b.Tax += item.Tax
		st.DebitNoteTotals[taxType] = b
		return fmt.Sprintf("0|ND Item|%s|%s|%s|%s|%s",
			desc, pyfmt.F3(qty), pyfmt.F2(price), pyfmt.F2(item.Tax), pyfmt.F2(item.Total)), nil
	}

	return "ERROR|0x50|Unknown item type", nil
}

// field returns raw[start:end] as a byte slice mapped onto Python's slicing.
func field(raw string, start, end int) string {
	r := []rune(raw)
	if start > len(r) {
		return ""
	}
	if end > len(r) {
		end = len(r)
	}
	return string(r[start:end])
}

// parseNoteItem reads qty(8.3) + price(10.2) + desc starting at offset 2.
func parseNoteItem(raw string, offset int, defaultDesc string) (qty, price float64, desc string, ok bool) {
	qtyStr := strings.TrimSpace(field(raw, offset, offset+8))
	priceStr := strings.TrimSpace(field(raw, offset+8, offset+18))
	desc = strings.TrimSpace(field(raw, offset+18, len(raw)))
	q, err := pyfmt.Float(qtyStr)
	if err != nil {
		return 0, 0, "", false
	}
	p, err := pyfmt.Float(priceStr)
	if err != nil {
		return 0, 0, "", false
	}
	if desc == "" {
		desc = defaultDesc
	}
	return q, p, desc, true
}

func taxName(idx int) string {
	switch idx {
	case 0:
		return "exempt"
	case 2:
		return "reduced"
	case 3:
		return "additional"
	}
	return "general"
}

func intParam(params []any, i, fallback int) int {
	if i >= len(params) || params[i] == nil {
		return fallback
	}
	switch v := params[i].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		n, err := strconv.Atoi(v)
		if err != nil {
			return fallback
		}
		return n
	}
	return fallback
}

// resolveTaxName accepts numeric codes, EN/ES names or a percentage, exactly
// like FiscalPrinterSimulator._resolve_tax_name.
func resolveTaxName(spec string) string {
	s := strings.ToLower(strings.TrimSpace(spec))
	nameMap := map[string]string{
		"0": "exempt", "exempt": "exempt", "exento": "exempt",
		"1": "general", "general": "general",
		"2": "reduced", "reduced": "reduced", "reducida": "reduced",
		"3": "additional", "additional": "additional",
		"adicional": "additional", "": "general",
	}
	if v, ok := nameMap[s]; ok {
		return v
	}
	value, err := pyfmt.Float(s)
	if err != nil {
		return "general"
	}
	switch value {
	case 0:
		return "exempt"
	case 16:
		return "general"
	case 8:
		return "reduced"
	case 30:
		return "additional"
	}
	return "general"
}

var _ = state.Item{}
