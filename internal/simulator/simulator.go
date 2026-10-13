// Package simulator routes protocol commands to their handlers and manages
// the virtual print-job queue.
package simulator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"simuladorfiscalgo/internal/persistence"
	"simuladorfiscalgo/internal/protocol"
	"simuladorfiscalgo/internal/render"
	"simuladorfiscalgo/internal/state"
)

// paymentMethodMap resolves the free text of a TFHKA "103 <texto>" command
// into the method recorded by the following "100" payment.
var paymentMethodMap = map[string]string{
	"cash": "cash", "efectivo": "cash", "money": "cash",
	"card": "card", "tarjeta": "card",
	"transfer": "transfer", "transferencia": "transfer",
	"credit": "credit", "credito": "credit", "crédito": "credit",
	"check": "check", "cheque": "check",
	"mobile": "mobile", "pago movil": "mobile",
}

// Simulator drives one virtual fiscal printer.
type Simulator struct {
	Brand        string
	Model        string
	State        *state.State
	CommandCount int
	History      []state.HistoryEntry
	ErrorLog     []map[string]any
	StatePath    string
}

// brandConfig mirrors FiscalPrinterSimulator._configure_brand.
type brandConfig struct {
	serial      string
	fiscalReg   string
	memoryTotal float64
}

var brandConfigs = map[string]brandConfig{
	"TFHKA":   {serial: "MLTFHKA001", fiscalReg: "J-12345678-9", memoryTotal: 4.0},
	"HASAR":   {serial: "MLHSR615F001", fiscalReg: "J-98765432-1", memoryTotal: 4.0},
	"BIXOLON": {serial: "MLBLN270001", fiscalReg: "J-55556666-3", memoryTotal: 2.0},
	"EPSON":   {serial: "MLSTM200001", fiscalReg: "J-44443333-5", memoryTotal: 8.0},
}

var defaultModels = map[string]string{
	"tfhka": "TFHKA", "hasar": "SMH/P-615F",
	"bixolon": "SRP-270", "epson": "TM2000",
}

// CreateSimulator builds a simulator for one of the four supported brands.
func CreateSimulator(brand, model, statePath string) (*Simulator, error) {
	key := strings.ToLower(brand)
	if _, ok := defaultModels[key]; !ok {
		return nil, fmt.Errorf("unknown brand '%s'. Supported: tfhka, hasar, bixolon, epson", brand)
	}
	if model == "" {
		model = defaultModels[key]
	}
	s := newSimulator(strings.ToUpper(key), model, statePath)
	return s, nil
}

func newSimulator(brand, model, statePath string) *Simulator {
	s := &Simulator{
		Brand:     brand,
		Model:     strings.ToUpper(model),
		State:     state.NewState(brand, model),
		StatePath: statePath,
	}
	s.configureBrand()
	if statePath != "" {
		persistence.Load(statePath, s.State, &s.CommandCount, &s.History)
	}
	return s
}

func (s *Simulator) configureBrand() {
	cfg, ok := brandConfigs[s.Brand]
	if !ok {
		cfg = brandConfigs["TFHKA"]
	}
	s.State.SerialNumber = cfg.serial
	s.State.FiscalRegistration = cfg.fiscalReg
	s.State.Rif = cfg.fiscalReg
	s.State.AuditMemoryTotal = cfg.memoryTotal
	s.State.AuditMemoryFree = cfg.memoryTotal * 0.9
	s.State.TaxRates = map[string]float64{
		"general": 16.0, "reduced": 8.0, "additional": 30.0,
	}
}

// ProcessCommand runs one protocol command, logs it and persists state.
func (s *Simulator) ProcessCommand(command string) string {
	s.CommandCount++

	s.State.Mu.Lock()
	defer s.State.Mu.Unlock()

	response, err := s.executeCommand(command)
	if err != nil {
		s.ErrorLog = append(s.ErrorLog, map[string]any{
			"command": command, "error": err.Error(),
			"timestamp": time.Now(),
		})
		response = s.formatResponse("ERROR|0x50|" + err.Error())
	} else {
		response = s.formatResponse(response)
	}

	s.logHistory(command, response)
	s.saveState()
	return response
}

// formatResponse applies the brand envelope; Epson wraps every response.
func (s *Simulator) formatResponse(response string) string {
	if s.Brand == "EPSON" {
		status := s.State.GetStatusCode()
		err := s.State.GetErrorCode()
		return fmt.Sprintf("0x%02X|0x%02X|%s", status, err, response)
	}
	return response
}

func (s *Simulator) logHistory(command, response string) {
	entry := state.HistoryEntry{
		Seq:      s.CommandCount,
		TS:       time.Now().Format("2006-01-02T15:04:05.000"),
		Command:  command,
		Response: response,
		OK:       !strings.Contains(response, "Error") && !strings.Contains(response, "ERROR"),
	}
	s.History = append(s.History, entry)
	if len(s.History) > 1000 {
		s.History = s.History[len(s.History)-1000:]
	}
}

func (s *Simulator) saveState() {
	if s.StatePath == "" {
		return
	}
	_ = persistence.Save(s.StatePath, s.Brand, s.Model, s.CommandCount, s.History, s.State)
}

// executeCommand routes a command to the right protocol handler.
func (s *Simulator) executeCommand(command string) (string, error) {
	// IxBatch @Commands (TFHKA's @COMENTARIO is not one of them).
	if strings.HasPrefix(command, "@") && !strings.HasPrefix(command, protocol.TFHKAComment) {
		ixcmd, ixparams, ok := protocol.ParseIxBatch(command)
		if !ok || ixcmd == "" {
			return "@Response|Error|0x50|Comando desconocido", nil
		}
		return s.handleIxBatch(ixcmd, ixparams)
	}

	cmdType, params := protocol.ParseTFHKA(command)
	if cmdType != "" && cmdType != "UNKNOWN" {
		return s.handleTFHKA(cmdType, params, command)
	}
	return "ERROR|0x50|Comando desconocido", nil
}

// ------------------------------------------------------------- paper jobs

// recordPaper appends a finished print job to the paper queue.
func (s *Simulator) recordPaper(kind, title string, lines []string, meta *state.JobMeta) *state.PrintJob {
	if meta == nil {
		meta = &state.JobMeta{}
	}
	job := &state.PrintJob{
		ID:    s.State.PrintJobSeq + 1,
		Kind:  kind,
		Title: title,
	TS:    time.Now().Format("2006-01-02T15:04:05"),
		Brand: s.Brand,
		Lines: lines,
		Meta:  meta,
	}
	s.State.PrintJobSeq++
	s.State.PrintJobs = append(s.State.PrintJobs, job)
	if len(s.State.PrintJobs) > 200 {
		s.State.PrintJobs = s.State.PrintJobs[len(s.State.PrintJobs)-200:]
	}
	return job
}

// recordReceipt renders a closed document and queues it as paper.
func (s *Simulator) recordReceipt(doc *state.Document) *state.PrintJob {
	company := doc.Company
	docType := doc.Type
	if docType == "" {
		docType = "invoice"
	}
	kind := docType
	if docType == "invoice" {
		kind = "receipt"
	}
	titles := map[string]string{
		"invoice":     fmt.Sprintf("FACTURA FISCAL N° %08d", doc.Number),
		"credit_note": fmt.Sprintf("NOTA DE CREDITO N° %08d", doc.Number),
		"debit_note":  fmt.Sprintf("NOTA DE DEBITO N° %08d", doc.Number),
		"non_fiscal":  fmt.Sprintf("DOC NO FISCAL N° %08d", doc.Number),
	}
	var lines []string
	if docType == "non_fiscal" {
		lines = render.RenderNonFiscal(doc, company)
	} else {
		lines = render.RenderReceipt(doc, company)
	}
	meta := state.NewReceiptMeta(doc.Number, doc.Total, docType)
	title := titles[docType]
	if title == "" {
		title = "DOCUMENTO"
	}
	return s.recordPaper(kind, title, lines, meta)
}

// recordReport renders an X or Z report and queues it as paper.
func (s *Simulator) recordReport(rep *state.Report, kind string) *state.PrintJob {
	var lines []string
	var title string
	if kind == "X" {
		lines = render.RenderXReport(rep, s.State.Company)
		title = fmt.Sprintf("REPORTE X N° %04d", rep.Number)
	} else {
		lines = render.RenderZReport(rep, s.State.Company)
		title = fmt.Sprintf("REPORTE Z N° %04d", rep.Number)
	}
	meta := state.NewReportMeta(rep.Number, rep.TotalSales, rep.TotalTax)
	return s.recordPaper(strings.ToLower(kind)+"_report", title, lines, meta)
}

// ------------------------------------------------------------- public API

// ProgramCompany programs company data into the fiscal memory.
func (s *Simulator) ProgramCompany(data map[string]any) state.Company {
	s.State.Mu.Lock()
	defer s.State.Mu.Unlock()
	result := s.State.ProgramCompany(data)
	s.saveState()
	return result
}

// CancelLocalDocument discards the open document without printing it.
func (s *Simulator) CancelLocalDocument() bool {
	s.State.Mu.Lock()
	defer s.State.Mu.Unlock()
	if !s.State.DocumentOpen {
		return false
	}
	s.State.DocumentOpen = false
	s.State.DocumentType = ""
	s.State.CurrentState = "waiting"
	s.State.CurrentItems = []state.Item{}
	s.State.SubtotalBases = 0
	s.State.SubtotalTax = 0
	s.State.AmountPayable = 0
	return true
}

// HistorySince returns entries with seq > since plus the command count.
func (s *Simulator) HistorySince(since int) ([]state.HistoryEntry, int) {
	s.State.Mu.Lock()
	defer s.State.Mu.Unlock()
	entries := []state.HistoryEntry{}
	for _, e := range s.History {
		if e.Seq > since {
			entries = append(entries, e)
		}
	}
	return entries, s.CommandCount
}

// PreviewLines renders the live preview of the open document.
func (s *Simulator) PreviewLines() (bool, []string) {
	s.State.Mu.Lock()
	defer s.State.Mu.Unlock()
	return s.State.DocumentOpen, render.RenderOpenDocument(s.State)
}

// JobsMeta returns print-job metadata, newest first.
func (s *Simulator) JobsMeta() []state.PrintJobMeta {
	s.State.Mu.Lock()
	defer s.State.Mu.Unlock()
	jobs := []state.PrintJobMeta{}
	for i := len(s.State.PrintJobs) - 1; i >= 0; i-- {
		j := s.State.PrintJobs[i]
		jobs = append(jobs, state.PrintJobMeta{
			ID: j.ID, Kind: j.Kind, Title: j.Title, TS: j.TS, Meta: j.Meta,
		})
	}
	return jobs
}

// FindJob returns a print job by id.
func (s *Simulator) FindJob(id int) (*state.PrintJob, bool) {
	s.State.Mu.Lock()
	defer s.State.Mu.Unlock()
	for _, j := range s.State.PrintJobs {
		if j.ID == id {
			return j, true
		}
	}
	return nil, false
}

// LastJobID returns the id of the newest print job, if any.
func (s *Simulator) LastJobID() (any, bool) {
	s.State.Mu.Lock()
	defer s.State.Mu.Unlock()
	if len(s.State.PrintJobs) == 0 {
		return nil, false
	}
	return s.State.PrintJobs[len(s.State.PrintJobs)-1].ID, true
}

// WithLock runs fn while holding the state lock (used by the web layer).
func (s *Simulator) WithLock(fn func()) {
	s.State.Mu.Lock()
	defer s.State.Mu.Unlock()
	fn()
}

// BrandUpper reports whether this simulator speaks the TFHKA ASCII protocol.
func (s *Simulator) IsTFHKA() bool { return s.Brand == "TFHKA" }

// writeJSON is a small helper kept for debugging state dumps.
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
