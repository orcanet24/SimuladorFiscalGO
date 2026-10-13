package simulator

import (
	"fmt"
	"strings"

	"simuladorfiscalgo/internal/pyfmt"
)

// handleIxBatch executes a parsed Hasar/Bixolon/Epson @Command.
func (s *Simulator) handleIxBatch(command string, params []string) (string, error) {
	st := s.State

	switch command {

	// ------------------------------------------------------ document open
	case "@OpenFiscalReceipt":
		st.OpenDocument("invoice")
		return "@Response|OK|0|Documento fiscal abierto", nil

	case "@OpenCreditNote":
		st.OpenDocument("credit_note")
		return "@Response|OK|0|Nota de credito abierta", nil

	case "@OpenDebitNote":
		st.OpenDocument("debit_note")
		return "@Response|OK|0|Nota de debito abierta", nil

	case "@OpenNonFiscalDoc":
		st.OpenDocument("non_fiscal")
		return "@Response|OK|0|Documento no fiscal abierto", nil

	case "@CloseNonFiscalDoc":
		if st.DocumentOpen {
			st.CloseDocument()
		}
		return "@Response|OK|0|DNF cerrado", nil

	// ------------------------------------------------------------ items
	// @PrintLineItem|desc|qty|price|tax_type|qualifier
	// @PrintLine|desc|qty|price|tax_name   (host/test-harness alias)
	case "@PrintLineItem", "@PrintLine":
		if !st.DocumentOpen {
			st.OpenDocument("invoice")
		}
		if len(params) >= 3 {
			desc := params[0]
			qty, err := pyfmt.Float(params[1])
			if err != nil {
				return "@Response|Error|2|Invalid params", nil
			}
			price, err := pyfmt.Float(params[2])
			if err != nil {
				return "@Response|Error|2|Invalid params", nil
			}
			taxSpec := "general"
			if len(params) > 3 {
				taxSpec = params[3]
			}
			taxName := resolveTaxName(taxSpec)
			item := st.AddItem(desc, qty, price, taxName)
			return fmt.Sprintf("@Response|OK|0|%s|%s|%s|%s",
				desc, pyfmt.F3(qty), pyfmt.F2(price), pyfmt.F2(item.Total)), nil
		}
		return "@Response|Error|2|Invalid params", nil

	case "@RefundItem":
		if !st.DocumentOpen {
			return "@Response|Error|1|No document open", nil
		}
		if len(params) >= 4 {
			desc := params[0]
			qty, err := pyfmt.Float(params[1])
			if err != nil {
				return "@Response|Error|2|Invalid params", nil
			}
			price, err := pyfmt.Float(params[2])
			if err != nil {
				return "@Response|Error|2|Invalid params", nil
			}
			taxName := resolveTaxName(params[3])
			item := st.AddItem(desc, qty, price, taxName)
			return fmt.Sprintf("@Response|OK|0|NC %s|%s|%s|%s",
				desc, pyfmt.F3(qty), pyfmt.F2(price), pyfmt.F2(item.Total)), nil
		}
		return "@Response|Error|2|Invalid params", nil

	// --------------------------------------------------------- customer
	case "@CustomerData", "@SetCustomerData":
		switch {
		case len(params) >= 3:
			st.CustomerRif = params[0]
			st.CustomerName = params[1]
			st.CustomerAddress = params[2]
		case len(params) == 2:
			st.CustomerRif = params[0]
			st.CustomerName = params[1]
		case len(params) == 1:
			st.CustomerRif = params[0]
		}
		return strings.TrimRight(fmt.Sprintf("@Response|OK|0|Cliente %s %s",
			st.CustomerRif, st.CustomerName), " "), nil

	case "@SetCustomerInfo1":
		if len(params) > 0 {
			st.CustomerName = params[0]
		}
		return "@Response|OK|0|Nombre registrado", nil

	case "@SetCustomerInfo2":
		if len(params) > 0 {
			st.CustomerNameLine2 = params[0]
		}
		return "@Response|OK|0|Nombre linea 2 registrada", nil

	case "@SetCustomerTIN":
		if len(params) > 0 {
			st.CustomerRif = params[0]
		}
		return "@Response|OK|0|RIF registrado", nil

	case "@SetCustomerExtraData":
		return "@Response|OK|0|Datos extra registrados", nil

	// --------------------------------------------------------- payments
	case "@Subtotal":
		return fmt.Sprintf("@Response|OK|0|Subtotal: %s+IVA:%s",
			pyfmt.F2(st.SubtotalBases), pyfmt.F2(st.SubtotalTax)), nil

	case "@AddPayment", "@Cash":
		if len(params) >= 1 {
			amount, err := pyfmt.Float(params[0])
			if err != nil {
				return "@Response|Error|2|Invalid params", nil
			}
			method := "cash"
			if len(params) > 1 {
				method = strings.ToLower(params[1])
			}
			methodMap := map[string]string{
				"efectivo": "cash", "cash": "cash", "money": "cash",
				"tarjeta": "card", "card": "card",
				"transferencia": "transfer", "transfer": "transfer",
				"credit": "credit", "credito": "credit",
				"cheque": "check", "check": "check",
			}
			if m, ok := methodMap[method]; ok {
				method = m
			}
			st.AddPayment(amount, method)
			return fmt.Sprintf("@Response|OK|0|Pago: %s|Pendiente: %s",
				pyfmt.F2(amount), pyfmt.F2(st.AmountPayable)), nil
		}
		return "@Response|Error|2|Invalid params", nil

	case "@TotalTender":
		if len(params) >= 2 {
			amount, err := pyfmt.Float(params[1])
			if err != nil {
				return "@Response|Error|2|Invalid params", nil
			}
			discount := 0.0
			if len(params) > 3 {
				discount, err = pyfmt.Float(params[3])
				if err != nil {
					return "@Response|Error|2|Invalid params", nil
				}
			}
			if discount > 0 {
				st.TotalDiscounts += discount
				st.AmountPayable -= discount
			}
			st.AddPayment(amount, "cash")
			change := st.AmountPayable * -1
			if change < 0 {
				change = 0
			}
			return fmt.Sprintf("@Response|OK|0|Pago: %s|Vuelto: %s|%08d",
				pyfmt.F2(amount), pyfmt.F2(change), st.InvoiceCounter+1), nil
		}
		return "@Response|Error|2|Invalid params", nil

	case "@DirectPayment":
		if len(params) >= 1 {
			st.AddPayment(st.AmountPayable, fmt.Sprintf("pay_%s", params[0]))
			return "@Response|OK|0|Pago directo", nil
		}
		return "@Response|Error|2|Invalid params", nil

	case "@RefundClose":
		if len(params) >= 2 {
			st.CloseDocument()
			return "@Response|OK|0|Devolucion cerrada: " + params[0], nil
		}
		return "@Response|Error|2|Invalid params", nil

	// ------------------------------------------------------ doc control
	case "@Cancel":
		if st.DocumentOpen {
			st.CloseDocument()
		}
		return "@Response|OK|0|Cancelado", nil

	case "@LastItemCancel":
		return "@Response|OK|0|Ultimo item cancelado", nil

	case "@LastItemDiscount":
		if len(params) >= 2 {
			amount, err := pyfmt.Float(params[0])
			if err != nil {
				return "@Response|Error|2|Invalid params", nil
			}
			return fmt.Sprintf("@Response|OK|0|Descuento: %s", pyfmt.F2(amount)), nil
		}
		return "@Response|Error|2|Invalid params", nil

	case "@Close", "@CloseFiscalReceipt":
		if !st.DocumentOpen {
			return "@Response|OK|0|Cerrado", nil
		}
		doc := st.CloseDocument()
		if doc != nil {
			job := s.recordReceipt(doc)
			name := map[string]string{
				"invoice": "Factura", "credit_note": "Nota de Credito",
				"debit_note": "Nota de Debito",
			}[doc.Type]
			if name == "" {
				name = "Documento"
			}
			return fmt.Sprintf("@Response|OK|0|%s %d cerrada|Total: %s|Impreso:%d",
				name, doc.Number, pyfmt.F2(doc.Total), job.ID), nil
		}
		return "@Response|OK|0|Cerrado", nil

	case "@CloseCashReceipt":
		if st.DocumentOpen {
			if doc := st.CloseDocument(); doc != nil {
				s.recordReceipt(doc)
			}
		}
		return "@Response|OK|0|DNF cerrado", nil

	// ---------------------------------------------------------- reports
	case "@PrintXReport":
		x := st.GenerateXReport()
		job := s.recordReport(x, "X")
		return fmt.Sprintf("@Response|OK|0|Reporte X #%d|Ventas: %s|Impreso:%d",
			x.Number, pyfmt.F2(x.TotalSales), job.ID), nil

	case "@PrintZReport":
		z := st.GenerateZReport()
		job := s.recordReport(z, "Z")
		return fmt.Sprintf("@Response|OK|0|Reporte Z #%d|Ventas: %s|Impreso:%d",
			z.Number, pyfmt.F2(z.TotalSales), job.ID), nil

	case "@DailyClose":
		if len(params) >= 1 {
			z := st.GenerateZReport()
			job := s.recordReport(z, "Z")
			return fmt.Sprintf("@Response|OK|0|Cierre %s #%d|Impreso:%d",
				params[0], z.Number, job.ID), nil
		}
		return "@Response|Error|2|Invalid params", nil

	case "@DailyCloseByDate":
		z := st.GenerateZReport()
		job := s.recordReport(z, "Z")
		return fmt.Sprintf("@Response|OK|0|Cierre por fecha #%d|Impreso:%d", z.Number, job.ID), nil

	case "@DailyCloseByNumber":
		z := st.GenerateZReport()
		job := s.recordReport(z, "Z")
		return fmt.Sprintf("@Response|OK|0|Cierre por numero #%d|Impreso:%d", z.Number, job.ID), nil

	case "@PrintAuditStatusReport":
		return "@Response|OK|0|Reporte de auditoria impreso", nil

	case "@Reprint":
		return "@Response|OK|0|Reimpresion generada", nil

	case "@ReprintByDate":
		return "@Response|OK|0|Reimpresion por fecha", nil

	case "@ReprintByNumber":
		return "@Response|OK|0|Reimpresion por numero", nil

	// ------------------------------------------------------ text / misc
	case "@PrintFiscalText":
		txt := ""
		if len(params) > 0 {
			txt = params[0]
		}
		return "@Response|OK|0|Texto: " + txt, nil

	case "@PrintNonFiscalText":
		txt := ""
		if len(params) > 0 {
			txt = params[0]
		}
		return "@Response|OK|0|No fiscal: " + txt, nil

	case "@PrintCashItem":
		return "@Response|OK|0|Movimiento caja", nil

	case "@BarCode":
		return "@Response|OK|0|Codigo barras impreso", nil

	// ---------------------------------------------------------- status
	case "@Status", "@StatusRequest":
		status := st.GetStatusCode()
		errCode := st.GetErrorCode()
		docState := "CERRADO"
		if st.DocumentOpen {
			docState = "ABIERTO"
		}
		return fmt.Sprintf("@Response|OK|0|Status: 0x%02X|Error: 0x%02X|Doc: %s|Ventas: %s",
			status, errCode, docState, pyfmt.F2(st.DailySales)), nil

	case "@StatusExtra":
		return "@Response|OK|0|Status extra OK", nil

	// -------------------------------------------------- configuration
	case "@ConfigureControllerByOne":
		return "@Response|OK|0|Configuracion OK", nil
	case "@PrintConfigurationData":
		return "@Response|OK|0|Configuracion impresa", nil
	case "@ProgramClerk":
		return "@Response|OK|0|Cajero programado", nil
	case "@ProgramPaymentMedia":
		return "@Response|OK|0|Medio pago programado", nil
	case "@ProgramSymbol":
		return "@Response|OK|0|Simbolo programado", nil
	case "@ProgramTaxes":
		return "@Response|OK|0|Tasas programadas", nil
	case "@SaveTaxes":
		return "@Response|OK|0|Tasas guardadas", nil
	case "@SetDate":
		return "@Response|OK|0|Fecha establecida", nil
	case "@SetTime":
		return "@Response|OK|0|Hora establecida", nil
	case "@Login":
		return "@Response|OK|0|Logon OK", nil
	case "@Logoff":
		return "@Response|OK|0|Logoff OK", nil
	case "@TrainingMode":
		return "@Response|OK|0|Modo entrenamiento", nil
	case "@OpenDrawer":
		drawer := "1"
		if len(params) > 0 {
			drawer = params[0]
		}
		return "@Response|OK|0|Gaveta " + drawer + " abierta", nil
	case "@PrintTest":
		return "@Response|OK|0|Test impreso", nil
	case "@ResetPrinterBuffer":
		return "@Response|OK|0|Buffer reseteado", nil
	case "@SendRawCommand":
		return "@Response|OK|0|Raw enviado", nil
	case "@SetHeaderTrailer":
		return "@Response|OK|0|Header/trailer configurado", nil

	// -------------------------------------------------------- display
	case "@DisplayCommercial":
		return "@Response|OK|0|Comercial mostrado", nil
	case "@DisplayDateTime":
		return "@Response|OK|0|Fecha/hora mostrada", nil
	case "@DisplayMessage":
		return "@Response|OK|0|Mensaje mostrado", nil
	case "@ProgramCommercial":
		return "@Response|OK|0|Comercial programado", nil
	case "@ProgramMessage":
		return "@Response|OK|0|Mensaje programado", nil

	// ----------------------------------------------------------- check
	case "@FormatCheck":
		return "@Response|OK|0|Cheque formateado", nil
	case "@FormatEndorse":
		return "@Response|OK|0|Endoso formateado", nil
	case "@ModeSlip":
		return "@Response|OK|0|Modo slip", nil
	case "@ModeValidation":
		return "@Response|OK|0|Validacion impresa", nil
	case "@PrintEndorse":
		return "@Response|OK|0|Endoso impreso", nil
	case "@PrintValidation":
		return "@Response|OK|0|Validacion impresa", nil
	case "@ReadMICR":
		return "@Response|OK|0|MICR leido", nil
	}

	return "@Response|Error|0x50|Comando desconocido", nil
}
