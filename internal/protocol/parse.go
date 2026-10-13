package protocol

import "strings"

// Result of parsing a TFHKA command: a command type, its parameters and
// whether anything was recognised at all. `nil` params mirror Python's [].
type TFHKAResult struct {
	Type   string
	Params []any
}

// ParseTFHKA parses a TFHKA ASCII command.
//
// This mirrors fiscalsim/protocol.py:parse_tfhka_command *exactly*, including
// the leading str.strip() which means an item whose tax prefix is a space
// (exempt) is stripped away and reported as UNKNOWN. That quirk is part of
// the reference behaviour and is asserted by the golden tests.
func ParseTFHKA(data string) (string, []any) {
	data = strings.TrimSpace(data)
	if data == "" {
		return "", nil
	}

	// Status commands
	switch data {
	case "S1", "S2", "S3", "S4", "S5", "S6", "S7", "S8E", "S8P":
		return data, nil
	}
	// Report commands
	switch data {
	case "U0X", "U1X", "U0Z", "U1Z":
		return data, nil
	}
	// Document control
	switch data {
	case TFHKASubtotalPrint:
		return "SUBTOTAL_PRINT", nil
	case TFHKASubtotalSilent:
		return "SUBTOTAL_SILENT", nil
	case TFHKACloseTotalize:
		return "CLOSE_TOTALIZE", nil
	case TFHKAOpenDrawer:
		return "OPEN_DRAWER", nil
	case TFHKACloseCreditNote:
		return "OPEN_CREDIT_NOTE", nil
	case TFHKACloseDebitNote:
		return "OPEN_DEBIT_NOTE", nil
	}

	// Credit/debit note item: d + taxType + qty + price + desc
	if strings.HasPrefix(data, "d") && len(data) > 2 {
		return "ITEM_CREDIT", []any{digitOr(data[1], 1)}
	}
	if strings.HasPrefix(data, "`") && len(data) > 2 {
		return "ITEM_DEBIT", []any{digitOr(data[1], 1)}
	}

	// Comment
	if strings.HasPrefix(data, TFHKAComment) {
		return "COMMENT", []any{strings.TrimSpace(data[len(TFHKAComment):])}
	}

	// Line item: prefix + qty + price + desc
	if c := data[0]; c == ' ' || c == '!' || c == '"' || c == '#' {
		switch c {
		case ' ':
			return "ITEM", []any{0}
		case '!':
			return "ITEM", []any{1}
		case '"':
			return "ITEM", []any{2}
		default:
			return "ITEM", []any{3}
		}
	}

	// Non-fiscal documents
	switch {
	case data == TFHKAOpenNonFiscal:
		return "OPEN_NON_FISCAL", nil
	case data == TFHKACloseNonFiscal:
		return "CLOSE_NON_FISCAL", nil
	case strings.HasPrefix(data, TFHKAPrintNonFiscalTx):
		return "PRINT_NON_FISCAL_TEXT", []any{data[3:]}
	case strings.HasPrefix(data, TFHKAPrintNonFiscalCt):
		return "PRINT_NON_FISCAL_CONTENT", []any{data[3:]}
	}

	// Payments, discounts, surcharges
	switch {
	case strings.HasPrefix(data, TFHKAPayment):
		return "PAYMENT", []any{data[3:]}
	case strings.HasPrefix(data, TFHKAPaymentDesc):
		return "PAYMENT_DESC", []any{data[3:]}
	case strings.HasPrefix(data, TFHKADiscount):
		return "DISCOUNT", []any{data[3:]}
	case strings.HasPrefix(data, TFHKASurcharge):
		return "SURCHARGE", []any{data[3:]}
	}

	// Customer / reference data
	switch {
	case strings.HasPrefix(data, "iR*"):
		return "CUSTOMER_RIF", []any{data[3:]}
	case strings.HasPrefix(data, "iS*"):
		return "CUSTOMER_NAME", []any{data[3:]}
	case strings.HasPrefix(data, "i01"):
		return "CUSTOMER_ADDRESS", []any{data[3:]}
	case strings.HasPrefix(data, "i02"):
		return "CUSTOMER_PHONE", []any{data[3:]}
	case strings.HasPrefix(data, "iF*"):
		return "INVOICE_NUMBER", []any{data[3:]}
	case strings.HasPrefix(data, "iD*"):
		return "INVOICE_DATE", []any{data[3:]}
	case strings.HasPrefix(data, "iI*"):
		return "FISCAL_SERIAL", []any{data[3:]}
	}

	return "UNKNOWN", []any{data}
}

func digitOr(b byte, fallback int) int {
	if b >= '0' && b <= '9' {
		return int(b - '0')
	}
	return fallback
}

// ParseIxBatch parses "@Command|param1|param2|...". Returns ok=false when the
// input does not start with '@'.
func ParseIxBatch(data string) (string, []string, bool) {
	if !strings.HasPrefix(data, "@") {
		return "", nil, false
	}
	parts := strings.Split(data, "|")
	command := strings.TrimSpace(parts[0])
	var params []string
	if len(parts) > 1 {
		for _, p := range parts[1:] {
			params = append(params, strings.TrimSpace(p))
		}
	}
	return command, params, true
}
