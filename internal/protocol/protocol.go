// Package protocol holds the TFHKA and IxBatch command constants, status
// tables, parsers and formatters shared by every simulated brand.
package protocol

import (
	"fmt"
	"strings"
)

// TFHKA document-control and report commands.
const (
	TFHKACloseCreditNote  = "d1"
	TFHKACloseDebitNote   = "d2"
	TFHKAOpenNonFiscal    = "80$"
	TFHKAPrintNonFiscalTx = "80!"
	TFHKAPrintNonFiscalCt = "80*"
	TFHKACloseNonFiscal   = "81"
	TFHKASubtotalPrint    = "3"
	TFHKASubtotalSilent   = "4"
	TFHKACloseTotalize    = "101"
	TFHKAOpenDrawer       = "0"
	TFHKAReportX          = "U0X"
	TFHKAReportX2         = "U1X"
	TFHKAReportZ          = "U0Z"
	TFHKAReportZ2         = "U1Z"
	TFHKAStatusS1         = "S1"
	TFHKAStatusS2         = "S2"
	TFHKAStatusS3         = "S3"
	TFHKAStatusS4         = "S4"
	TFHKAStatusS5         = "S5"
	TFHKAStatusS8E        = "S8E"
	TFHKAStatusS8P        = "S8P"
	TFHKAPayment          = "100"
	TFHKAPaymentDesc      = "103"
	TFHKADiscount         = "700"
	TFHKASurcharge        = "701"
	TFHKAComment          = "@COMENTARIO"
)

// IxBatch command names (Hasar / Bixolon / Epson).
const (
	IxOpenFiscalReceipt  = "@OpenFiscalReceipt"
	IxOpenCreditNote     = "@OpenCreditNote"
	IxOpenDebitNote      = "@OpenDebitNote"
	IxOpenNonFiscalDoc   = "@OpenNonFiscalDoc"
	IxPrintLine          = "@PrintLine"
	IxPrintLineItem      = "@PrintLineItem"
	IxSubtotal           = "@Subtotal"
	IxAddPayment         = "@AddPayment"
	IxCash               = "@Cash"
	IxClose              = "@Close"
	IxCloseFiscalReceipt = "@CloseFiscalReceipt"
	IxCloseNonFiscalDoc  = "@CloseNonFiscalDoc"
	IxPrintXReport       = "@PrintXReport"
	IxPrintZReport       = "@PrintZReport"
	IxStatus             = "@Status"
	IxCustomerData       = "@CustomerData"
)

// STATUS_CODES mirrors the TFHKA Anexo 1 table (keyed by status byte).
var STATUS_CODES = map[byte]string{
	0x00: "Estado desconocido",
	0x01: "Modo prueba - en espera",
	0x02: "Modo prueba - emision fiscal",
	0x03: "Modo prueba - emision no fiscal",
	0x04: "Modo fiscal - en espera",
	0x05: "Modo fiscal - emision fiscal",
	0x06: "Modo fiscal - emision no fiscal",
	0x07: "Fiscal casi llena - en espera",
	0x08: "Fiscal casi llena - emision fiscal",
	0x09: "Fiscal casi llena - emision no fiscal",
	0x0A: "Memoria fiscal llena - en espera",
	0x0B: "Memoria fiscal llena - emision fiscal",
	0x0C: "Memoria fiscal llena - emision no fiscal",
}

// ERROR_CODES mirrors the TFHKA Anexo 2 table (keyed by error byte).
var ERROR_CODES = map[byte]string{
	0x00: "Sin error",
	0x01: "Fin papel",
	0x02: "Error mecanico papel",
	0x03: "Fin papel + error mecanico",
	0x50: "Comando invalido",
	0x54: "Tasa invalida",
	0x58: "Sin directivas",
	0x5C: "Comando invalido",
	0x60: "Error fiscal",
	0x64: "Error memoria fiscal",
	0x6C: "Memoria fiscal llena",
	0x70: "Buffer completo - reiniciar",
	0x80: "Error comunicacion",
	0x89: "Sin respuesta",
	0x90: "Error LRC",
	0x91: "Error interno API",
	0x99: "Error apertura archivo",
}

// GetStatusByte maps (mode, state, memory) onto the TFHKA status byte.
func GetStatusByte(mode, state, memory string) byte {
	if mode == "test" {
		switch state {
		case "waiting":
			return 0x01
		case "fiscal":
			return 0x02
		case "non_fiscal":
			return 0x03
		}
		return 0x04
	}
	switch state {
	case "waiting":
		switch memory {
		case "almost_full":
			return 0x07
		case "full":
			return 0x0A
		}
		return 0x04
	case "fiscal":
		switch memory {
		case "almost_full":
			return 0x08
		case "full":
			return 0x0B
		}
		return 0x05
	case "non_fiscal":
		switch memory {
		case "almost_full":
			return 0x09
		case "full":
			return 0x0C
		}
		return 0x06
	}
	return 0x04
}

// FormatLineItem renders Prefix(1) + Qty(8.3) + Price(10.2) + description.
func FormatLineItem(prefix string, qty, price float64, description string) string {
	return fmt.Sprintf("%s%8.3f%10.2f%s", prefix, qty, price, description)
}

// FormatCreditNoteItem renders d + taxType + qty(8.3) + price(10.2) + desc.
func FormatCreditNoteItem(taxType int, qty, price float64, description string) string {
	return fmt.Sprintf("d%d%8.3f%10.2f%s", taxType, qty, price, description)
}

// FormatDebitNoteItem renders ` + taxType + qty(8.3) + price(10.2) + desc.
func FormatDebitNoteItem(taxType int, qty, price float64, description string) string {
	return fmt.Sprintf("`%d%8.3f%10.2f%s", taxType, qty, price, description)
}

// LineItemPrefix maps a tax rate onto its TFHKA prefix character.
func LineItemPrefix(taxRate float64) string {
	switch {
	case taxRate == 0:
		return " "
	case taxRate < 12:
		return `" `
	case taxRate >= 16:
		return "#"
	}
	return "!"
}

// LineItemPrefixFor builds the prefix for a named tax type used by the panel.
func LineItemPrefixFor(taxType string) string {
	switch strings.ToLower(taxType) {
	case "exempt", "exento", "0":
		return " "
	case "reduced", "reducida", "2":
		return `"`
	case "additional", "adicional", "3":
		return "#"
	}
	return "!"
}
