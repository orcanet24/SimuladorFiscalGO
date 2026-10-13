// Package web serves the HTTP API and the static browser panel.
package web

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"simuladorfiscalgo/internal/orderedmap"
	"simuladorfiscalgo/internal/pyfmt"
	"simuladorfiscalgo/internal/protocol"
	"simuladorfiscalgo/internal/simulator"
	"simuladorfiscalgo/internal/state"
)

//go:embed static/*
var staticFS embed.FS

// Brands served by the panel, in tab order.
var Brands = []string{"tfhka", "hasar", "bixolon", "epson"}

// tfhkaPrefix maps a panel tax selection onto its TFHKA prefix character.
var tfhkaPrefix = map[string]string{
	"exempt": " ", "general": "!", "reduced": `"`, "additional": "#",
}

// Panel holds one simulator per brand plus the shared data directory.
type Panel struct {
	Sims    map[string]*simulator.Simulator
	DataDir string
}

// NewPanel creates the four brand simulators and loads their persisted state.
func NewPanel(dataDir string) (*Panel, error) {
	if dataDir == "" {
		dataDir = "data"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	p := &Panel{
		Sims:    map[string]*simulator.Simulator{},
		DataDir: dataDir,
	}
	for _, brand := range Brands {
		path := filepath.Join(dataDir, "state_"+brand+".json")
		sim, err := simulator.CreateSimulator(brand, "", path)
		if err != nil {
			return nil, err
		}
		p.Sims[brand] = sim
	}
	return p, nil
}

// Get returns the simulator for a brand, falling back to tfhka.
func (p *Panel) Get(brand string) *simulator.Simulator {
	brand = strings.ToLower(brand)
	if brand == "" {
		brand = "tfhka"
	}
	if s, ok := p.Sims[brand]; ok {
		return s
	}
	return p.Sims["tfhka"]
}

// ---------------------------------------------------------------- payloads

type statePayload struct {
	Brand        string         `json:"brand"`
	Model        string         `json:"model"`
	CommandCount int            `json:"command_count"`
	Status       codePayload    `json:"status"`
	Error        codePayload    `json:"error"`
	Mode         string         `json:"mode"`
	MemoryStatus string         `json:"memory_status"`
	Document     docPayload     `json:"document"`
	Counters     counterPayload `json:"counters"`
	Daily        dailyPayload   `json:"daily"`
	Totals       totalsPayload  `json:"totals"`
	Payments     map[string]any           `json:"payments"`
	TaxRates     *orderedmap.Map[pyfmt.PyFloat] `json:"tax_rates"`
	SerialNumber string         `json:"serial_number"`
	Rif          string         `json:"rif"`
	Memory       memoryPayload  `json:"memory"`
	PrintJobSeq  int            `json:"print_job_seq"`
	Headers      []string       `json:"headers"`
	IsTFHKA      bool           `json:"is_tfhka"`
}

type codePayload struct {
	Code int    `json:"code"`
	Hex  string `json:"hex"`
	Name string `json:"name"`
}

type docPayload struct {
	Open     bool            `json:"open"`
	Type     any             `json:"type"`
	Items    int             `json:"items"`
	Subtotal pyfmt.PyFloat   `json:"subtotal"`
	Payable  pyfmt.PyFloat   `json:"payable"`
	Paid     pyfmt.PyFloat   `json:"paid"`
	Customer customerPayload `json:"customer"`
}

type customerPayload struct {
	Rif     string `json:"rif"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

type counterPayload struct {
	Invoices      int `json:"invoices"`
	CreditNotes   int `json:"credit_notes"`
	DebitNotes    int `json:"debit_notes"`
	NonFiscal     int `json:"non_fiscal"`
	XReports      int `json:"x_reports"`
	ZReports      int `json:"z_reports"`
	DailyClosure  int `json:"daily_closure"`
	FiscalDocs    int `json:"fiscal_docs"`
}

type dailyPayload struct {
	Invoices     int     `json:"invoices"`
	CreditNotes  int     `json:"credit_notes"`
	DebitNotes   int     `json:"debit_notes"`
	NonFiscal    int          `json:"non_fiscal"`
	Sales        pyfmt.PyFloat `json:"sales"`
	Tax          pyfmt.PyFloat `json:"tax"`
}

type totalsPayload struct {
	LifetimeSales pyfmt.PyFloat `json:"lifetime_sales"`
	LifetimeTax   pyfmt.PyFloat `json:"lifetime_tax"`
}

type memoryPayload struct {
	TotalMB pyfmt.PyFloat `json:"total_mb"`
	FreeMB  pyfmt.PyFloat `json:"free_mb"`
	UsedMB  pyfmt.PyFloat `json:"used_mb"`
	UsedPct pyfmt.PyFloat `json:"used_pct"`
}

// buildStatePayload mirrors webapp.build_state_payload.
func buildStatePayload(sim *simulator.Simulator) statePayload {
	st := sim.State
	status := st.GetStatusCode()
	errCode := st.GetErrorCode()

	statusName, ok := protocol.STATUS_CODES[status]
	if !ok {
		statusName = "Desconocido"
	}
	errName, ok := protocol.ERROR_CODES[errCode]
	if !ok {
		errName = "Desconocido"
	}

	var docType any
	if st.DocumentType != "" {
		docType = st.DocumentType
	}

	payments := map[string]any{}
	st.PaymentTotals.ForEach(func(k string, v float64) {
		payments[k] = pyfmt.PyFloat(pyfmt.PyRound(v, 2))
	})

	// tax_rates keeps the reference dict's insertion order.
	taxRates := orderedmap.New[pyfmt.PyFloat]()
	for _, k := range []string{"general", "reduced", "additional"} {
		taxRates.Set(k, pyfmt.PyFloat(st.TaxRates[k]))
	}

	total := st.AuditMemoryTotal
	used := st.AuditMemoryUsed
	denom := total
	if denom < 0.001 {
		denom = 0.001
	}

	return statePayload{
		Brand:        sim.Brand,
		Model:        sim.Model,
		CommandCount: sim.CommandCount,
		Status:       codePayload{Code: int(status), Hex: fmt.Sprintf("0x%02X", status), Name: statusName},
		Error:        codePayload{Code: int(errCode), Hex: fmt.Sprintf("0x%02X", errCode), Name: errName},
		Mode:         st.Mode,
		MemoryStatus: st.MemoryStatus,
		Document: docPayload{
			Open:     st.DocumentOpen,
			Type:     docType,
			Items:    len(st.CurrentItems),
			Subtotal: pyfmt.PyFloat(pyfmt.PyRound(st.SubtotalBases+st.SubtotalTax, 2)),
			Payable:  pyfmt.PyFloat(pyfmt.PyRound(st.AmountPayable, 2)),
			Paid:     pyfmt.PyFloat(pyfmt.PyRound(st.PaymentsMade, 2)),
			Customer: customerPayload{
				Rif: st.CustomerRif, Name: st.CustomerName, Address: st.CustomerAddress,
			},
		},
		Counters: counterPayload{
			Invoices: st.InvoiceCounter, CreditNotes: st.CreditNoteCounter,
			DebitNotes: st.DebitNoteCounter, NonFiscal: st.NonFiscalCounter,
			XReports: st.XReportCounter, ZReports: st.ZReportCounter,
			DailyClosure: st.DailyClosureCounter, FiscalDocs: st.FiscalCounter,
		},
		Daily: dailyPayload{
			Invoices: st.DailyInvoices, CreditNotes: st.DailyCreditNotes,
			DebitNotes: st.DailyDebitNotes, NonFiscal: st.DailyNonFiscal,
			Sales: pyfmt.PyFloat(pyfmt.PyRound(st.DailySales, 2)),
			Tax:   pyfmt.PyFloat(pyfmt.PyRound(st.DailyTax, 2)),
		},
		Totals: totalsPayload{
			LifetimeSales: pyfmt.PyFloat(pyfmt.PyRound(st.TotalSales, 2)),
			LifetimeTax:   pyfmt.PyFloat(pyfmt.PyRound(st.TotalTax, 2)),
		},
		Payments:     payments,
		TaxRates:     taxRates,
		SerialNumber: st.SerialNumber,
		Rif:          st.Rif,
		Memory: memoryPayload{
			TotalMB: pyfmt.PyFloat(total),
			FreeMB:  pyfmt.PyFloat(pyfmt.PyRound(st.AuditMemoryFree, 2)),
			UsedMB:  pyfmt.PyFloat(pyfmt.PyRound(used, 2)),
			UsedPct: pyfmt.PyFloat(pyfmt.PyRound(100*used/denom, 1)),
		},
		PrintJobSeq: st.PrintJobSeq,
		Headers:     st.Headers,
		IsTFHKA:     sim.IsTFHKA(),
	}
}

// ------------------------------------------------------------- host actions

type actionResponse struct {
	Commands  []string `json:"commands"`
	Responses []cmdResp `json:"responses"`
	LastJobID any      `json:"last_job_id"`
	Error     string   `json:"error,omitempty"`
	Note      string   `json:"note,omitempty"`
}

type cmdResp struct {
	Command  string `json:"command"`
	Response string `json:"response"`
}

// hostAction translates a structured panel action into real protocol commands.
func hostAction(sim *simulator.Simulator, action string, params map[string]any) actionResponse {
	isTFHKA := sim.IsTFHKA()
	var commands []string

	str := func(key, def string) string {
		if v, ok := params[key].(string); ok {
			return v
		}
		return def
	}
	num := func(key string, def float64) float64 {
		switch v := params[key].(type) {
		case float64:
			return v
		case string:
			if f, err := pyfmt.Float(v); err == nil {
				return f
			}
		}
		return def
	}

	switch action {
	case "open":
		doc := str("doc", "invoice")
		rif := str("rif", "")
		name := str("name", "")
		address := str("address", "")
		if isTFHKA {
			if rif != "" {
				commands = append(commands, "iR*"+rif)
			}
			if name != "" {
				commands = append(commands, "iS*"+name)
			}
			if address != "" {
				commands = append(commands, "i01"+address)
			}
			if doc == "credit_note" {
				commands = append(commands, "d1")
			} else if doc == "debit_note" {
				commands = append(commands, "d2")
			}
		} else {
			switch doc {
			case "credit_note":
				commands = append(commands, "@OpenCreditNote")
			case "debit_note":
				commands = append(commands, "@OpenDebitNote")
			default:
				commands = append(commands, "@OpenFiscalReceipt")
			}
			if rif != "" || name != "" || address != "" {
				commands = append(commands,
					fmt.Sprintf("@CustomerData|%s|%s|%s", rif, name, address))
			}
		}

	case "item":
		desc := str("description", "ITEM")
		qty := num("qty", 1)
		price := num("price", 0)
		tax := str("tax", "general")
		if isTFHKA {
			prefix := tfhkaPrefix[tax]
			if prefix == "" {
				prefix = "!"
			}
			commands = append(commands, fmt.Sprintf("%s%8.3f%10.2f%s", prefix, qty, price, desc))
		} else {
			commands = append(commands,
				fmt.Sprintf("@PrintLine|%s|%s|%s|%s", desc, trimFloat(qty), trimFloat(price), tax))
		}

	case "subtotal":
		if isTFHKA {
			commands = append(commands, "3")
		} else {
			commands = append(commands, "@Subtotal")
		}

	case "payment":
		amount := num("amount", 0)
		method := str("method", "cash")
		if isTFHKA {
			// TFHKA declares the tender type with 103 before the 100 amount.
			if method != "" && method != "cash" {
				commands = append(commands, "103"+method)
			}
			commands = append(commands, fmt.Sprintf("100%.2f", amount))
		} else {
			commands = append(commands, fmt.Sprintf("@AddPayment|%.2f|%s", amount, method))
		}

	case "close":
		if isTFHKA {
			commands = append(commands, "101")
		} else {
			commands = append(commands, "@Close")
		}

	case "status":
		if isTFHKA {
			commands = append(commands, "S1")
		} else {
			commands = append(commands, "@Status")
		}

	case "cancel":
		if isTFHKA {
			// TFHKA has no direct cancel: discard the document locally.
			if sim.CancelLocalDocument() {
				return actionResponse{
					Commands: []string{}, Responses: []cmdResp{},
					Note: "Documento cancelado localmente",
				}
			}
			return actionResponse{
				Commands: []string{}, Responses: []cmdResp{}, Note: "Sin documento",
			}
		}
		commands = append(commands, "@Cancel")

	case "report":
		kind := strings.ToLower(str("type", "x"))
		if isTFHKA {
			if kind == "x" {
				commands = append(commands, "U0X")
			} else {
				commands = append(commands, "U0Z")
			}
		} else {
			if kind == "x" {
				commands = append(commands, "@PrintXReport")
			} else {
				commands = append(commands, "@PrintZReport")
			}
		}

	case "drawer":
		if isTFHKA {
			commands = append(commands, "0")
		} else {
			commands = append(commands, "@OpenDrawer")
		}

	case "customer":
		rif := str("rif", "")
		name := str("name", "")
		address := str("address", "")
		if isTFHKA {
			if rif != "" {
				commands = append(commands, "iR*"+rif)
			}
			if name != "" {
				commands = append(commands, "iS*"+name)
			}
			if address != "" {
				commands = append(commands, "i01"+address)
			}
		} else {
			commands = append(commands,
				fmt.Sprintf("@CustomerData|%s|%s|%s", rif, name, address))
		}

	default:
		return actionResponse{Error: "Acción desconocida: " + action}
	}

	responses := make([]cmdResp, 0, len(commands))
	for _, cmd := range commands {
		responses = append(responses, cmdResp{Command: cmd, Response: sim.ProcessCommand(cmd)})
	}
	return actionResponse{
		Commands:  commands,
		Responses: responses,
		LastJobID: lastJobID(sim),
	}
}

func lastJobID(sim *simulator.Simulator) any {
	if id, ok := sim.LastJobID(); ok {
		return id
	}
	return nil
}

// trimFloat renders a number the way the Python f-string did: "%s" of a float.
func trimFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// --------------------------------------------------------------- HTTP glue

// historyPayload mirrors the reference dict literal order (entries, count).
type historyPayload struct {
	Entries []state.HistoryEntry `json:"entries"`
	Count   int                  `json:"count"`
}

// previewPayload mirrors the reference dict literal order (open, lines).
type previewPayload struct {
	Open  bool     `json:"open"`
	Lines []string `json:"lines"`
}

// writeJSON serializes the payload exactly like the reference implementation:
// json.dumps(payload, ensure_ascii=False) - UTF-8 raw, no HTML escaping,
// separators ", " and ": ", no trailing newline.
func writeJSON(w http.ResponseWriter, payload any, status int) {
	body, err := marshalPy(payload)
	if err != nil {
		http.Error(w, `{"error":"marshal"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// marshalPy renders JSON with Python's default separators.
func marshalPy(payload any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		return nil, err
	}
	b := bytes.TrimRight(buf.Bytes(), "\n")
	return insertPySpaces(b), nil
}

// insertPySpaces adds the space Python writes after ':' and ',' outside of
// strings (the compact encoder output has none).
func insertPySpaces(b []byte) []byte {
	out := make([]byte, 0, len(b)+len(b)/8)
	inStr, esc := false, false
	for _, c := range b {
		out = append(out, c)
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
		} else if c == ':' || c == ',' {
			out = append(out, ' ')
		}
	}
	return out
}

func readBody(r *http.Request) map[string]any {
	if r.Body == nil {
		return map[string]any{}
	}
	var out map[string]any
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&out); err != nil {
		return map[string]any{}
	}
	return out
}

// Handler builds the panel's http.Handler.
func Handler(p *Panel) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			writeJSON(w, map[string]string{"error": "not found"}, http.StatusNotFound)
			return
		}
		serveFile(w, "static/index.html", "text/html; charset=utf-8")
	})

	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, buildStatePayload(p.Get(r.URL.Query().Get("brand"))), http.StatusOK)
	})

	mux.HandleFunc("GET /api/history", func(w http.ResponseWriter, r *http.Request) {
		sim := p.Get(r.URL.Query().Get("brand"))
		since := queryInt(r, "since", 0)
		entries, count := sim.HistorySince(since)
		writeJSON(w, historyPayload{Entries: entries, Count: count}, http.StatusOK)
	})

	mux.HandleFunc("GET /api/preview", func(w http.ResponseWriter, r *http.Request) {
		sim := p.Get(r.URL.Query().Get("brand"))
		open, lines := sim.PreviewLines()
		if lines == nil {
			lines = []string{}
		}
		writeJSON(w, previewPayload{Open: open, Lines: lines}, http.StatusOK)
	})

	mux.HandleFunc("GET /api/jobs", func(w http.ResponseWriter, r *http.Request) {
		sim := p.Get(r.URL.Query().Get("brand"))
		writeJSON(w, map[string]any{"jobs": sim.JobsMeta()}, http.StatusOK)
	})

	mux.HandleFunc("GET /api/job/{id}", func(w http.ResponseWriter, r *http.Request) {
		sim := p.Get(r.URL.Query().Get("brand"))
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeJSON(w, map[string]string{"error": "bad id"}, http.StatusBadRequest)
			return
		}
		job, ok := sim.FindJob(id)
		if !ok {
			writeJSON(w, map[string]string{"error": "job not found"}, http.StatusNotFound)
			return
		}
		writeJSON(w, job, http.StatusOK)
	})

	mux.HandleFunc("GET /api/company", func(w http.ResponseWriter, r *http.Request) {
		sim := p.Get(r.URL.Query().Get("brand"))
		writeJSON(w, map[string]any{
			"company": sim.State.Company,
			"headers": sim.State.Headers,
		}, http.StatusOK)
	})

	mux.HandleFunc("POST /api/command", func(w http.ResponseWriter, r *http.Request) {
		body := readBody(r)
		sim := p.Get(brandOf(body))
		command, _ := body["command"].(string)
		command = strings.TrimSpace(command)
		if command == "" {
			writeJSON(w, map[string]string{"error": "command required"}, http.StatusBadRequest)
			return
		}
		response := sim.ProcessCommand(command)
		writeJSON(w, map[string]any{
			"command": command, "response": response, "seq": sim.CommandCount,
		}, http.StatusOK)
	})

	mux.HandleFunc("POST /api/company", func(w http.ResponseWriter, r *http.Request) {
		body := readBody(r)
		sim := p.Get(brandOf(body))
		data, _ := body["company"].(map[string]any)
		if data == nil {
			data, _ = body["data"].(map[string]any)
		}
		if len(data) == 0 {
			writeJSON(w, map[string]string{"error": "company data required"}, http.StatusBadRequest)
			return
		}
		result := sim.ProgramCompany(data)
		writeJSON(w, map[string]any{
			"company": result, "headers": sim.State.Headers,
		}, http.StatusOK)
	})

	mux.HandleFunc("POST /api/sale", func(w http.ResponseWriter, r *http.Request) {
		body := readBody(r)
		sim := p.Get(brandOf(body))
		action, _ := body["action"].(string)
		action = strings.TrimSpace(action)
		if action == "" {
			writeJSON(w, map[string]string{"error": "action required"}, http.StatusBadRequest)
			return
		}
		params, _ := body["params"].(map[string]any)
		if params == nil {
			params = map[string]any{}
		}
		writeJSON(w, hostAction(sim, action, params), http.StatusOK)
	})

	mux.HandleFunc("POST /api/report", func(w http.ResponseWriter, r *http.Request) {
		body := readBody(r)
		sim := p.Get(brandOf(body))
		kind, _ := body["type"].(string)
		writeJSON(w, hostAction(sim, "report", map[string]any{"type": kind}), http.StatusOK)
	})

	mux.HandleFunc("POST /{rest}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"error": "not found"}, http.StatusNotFound)
	})

	return noStore(mux)
}

// noStore mirrors the reference's Cache-Control: no-store on every response.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func serveFile(w http.ResponseWriter, name, contentType string) {
	data, err := staticFS.ReadFile(name)
	if err != nil {
		writeJSON(w, map[string]string{"error": "not found"}, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func brandOf(body map[string]any) string {
	if b, ok := body["brand"].(string); ok {
		return b
	}
	return "tfhka"
}

func queryInt(r *http.Request, key string, def int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}
