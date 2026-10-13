package simulator

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"simuladorfiscalgo/internal/protocol"
)

var paperTSRe = regexp.MustCompile(`\d{2}/\d{2}/\d{4}\s+\d{2}:\d{2}:\d{2}`)

func normalizePaper(s string) string {
	return paperTSRe.ReplaceAllString(s, "<TS>")
}

// itemCmd builds a TFHKA line item with the real fixed-width protocol
// formatter, so the field alignment matches what the golden generator used.
func itemCmd(desc string, qty, price float64, tax string) string {
	return protocol.FormatLineItem(protocol.LineItemPrefixFor(tax), qty, price, desc)
}

// jobFlow is one scripted session whose printed paper must match Python's.
type jobFlow struct {
	brand string
	cmds  []string
	file  string
}

var jobFlows = []jobFlow{
	{"tfhka", []string{
		"iR*J-50123456-1", "iS*COMERCIAL EL COMUNERO, C.A.",
		"i01CALLE REAL DE CHACAO, NRO 15-23",
		itemCmd("ARROZ VENEZOLANO 1KG", 6, 2.50, "general"),
		itemCmd("ACEITE VEGETAL 1L", 3, 4.90, "general"),
		itemCmd("LECHE EN POLVO 1KG", 2, 7.50, "reduced"),
		itemCmd("PAN CASERO", 4, 1.20, "exempt"),
		"3", "10066.35", "101",
		itemCmd("CAFE", 1, 15.00, "general"), "3", "103card", "10020.00", "101",
		"d1", "iF*00000001",
		protocol.FormatCreditNoteItem(1, 1, 15.00, "CAFE"), "101",
		"80$", "80!PRESUPUESTO", "81",
		"U0X", "U0Z",
	}, "paper_jobs_tfhka.jsonl"},
	{"hasar", []string{
		"@OpenFiscalReceipt",
		"@CustomerData|J-50123456-1|COMERCIAL EL COMUNERO, C.A.|CALLE REAL 15-23",
		"@PrintLine|ARROZ VENEZOLANO 1KG|6|2.50|general",
		"@PrintLine|ACEITE VEGETAL 1L|3|4.90|general",
		"@PrintLine|LECHE EN POLVO 1KG|2|7.50|reduced",
		"@PrintLine|PAN CASERO|4|1.20|exempt",
		"@Subtotal", "@AddPayment|66.35|cash", "@Close",
		"@PrintXReport", "@PrintZReport",
	}, "paper_jobs_hasar.jsonl"},
	{"bixolon", []string{
		"@OpenFiscalReceipt",
		"@CustomerData|J-50123456-1|COMERCIAL EL COMUNERO, C.A.|CALLE REAL 15-23",
		"@PrintLine|ARROZ VENEZOLANO 1KG|6|2.50|general",
		"@PrintLine|ACEITE VEGETAL 1L|3|4.90|general",
		"@PrintLine|LECHE EN POLVO 1KG|2|7.50|reduced",
		"@PrintLine|PAN CASERO|4|1.20|exempt",
		"@Subtotal", "@AddPayment|66.35|cash", "@Close",
		"@PrintXReport", "@PrintZReport",
	}, "paper_jobs_bixolon.jsonl"},
	{"epson", []string{
		"@OpenFiscalReceipt",
		"@CustomerData|J-50123456-1|COMERCIAL EL COMUNERO, C.A.|CALLE REAL 15-23",
		"@PrintLine|ARROZ VENEZOLANO 1KG|6|2.50|general",
		"@PrintLine|ACEITE VEGETAL 1L|3|4.90|general",
		"@PrintLine|LECHE EN POLVO 1KG|2|7.50|reduced",
		"@PrintLine|PAN CASERO|4|1.20|exempt",
		"@Subtotal", "@AddPayment|66.35|cash", "@Close",
		"@PrintXReport", "@PrintZReport",
	}, "paper_jobs_epson.jsonl"},
}

type jobGolden struct {
	I        int             `json:"i"`
	Command  string          `json:"command"`
	Response string          `json:"response"`
	Job      json.RawMessage `json:"job"`
}

type jobPayload struct {
	ID    int             `json:"id"`
	Kind  string          `json:"kind"`
	Title string          `json:"title"`
	Lines []string        `json:"lines"`
	Meta  json.RawMessage `json:"meta"`
}

func TestPaperJobsGolden(t *testing.T) {
	for _, flow := range jobFlows {
		t.Run(flow.brand, func(t *testing.T) {
			want := readJobs(t, flow.file)
			got := runFlow(flow)
			if len(got) != len(want) {
				t.Fatalf("step count: got %d, want %d", len(got), len(want))
			}
			for i := range want {
				if got[i].Command != want[i].Command {
					t.Errorf("[%d] command: got %q, want %q", i, got[i].Command, want[i].Command)
					continue
				}
				if got[i].Response != want[i].Response {
					t.Errorf("[%d] %q\n got  %q\n want %q",
						i, want[i].Command, got[i].Response, want[i].Response)
				}
				compareJob(t, i, got[i].Job, want[i].Job)
			}
		})
	}
}

func readJobs(t *testing.T, file string) []jobGolden {
	t.Helper()
	var out []jobGolden
	for _, line := range readLines(t, file) {
		var rec jobGolden
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("bad jsonl: %v", err)
		}
		out = append(out, rec)
	}
	return out
}

func runFlow(flow jobFlow) []jobGolden {
	sim, err := CreateSimulator(flow.brand, "", "")
	if err != nil {
		panic(err)
	}
	sim.ProcessCommand("S1")

	var out []jobGolden
	for i, cmd := range flow.cmds {
		resp := sim.ProcessCommand(cmd)
		rec := jobGolden{I: i, Command: cmd, Response: normalizeTS(resp)}

		// The Python harness attaches the job referenced by "Impreso:<id>".
		if idx := strings.LastIndex(resp, "Impreso:"); idx >= 0 {
			idStr := resp[idx+len("Impreso:"):]
			var id int
			if _, err := fmtSscanInt(idStr, &id); err == nil {
				if job, ok := sim.FindJob(id); ok {
					lines := make([]string, len(job.Lines))
					for j, l := range job.Lines {
						lines[j] = normalizePaper(l)
					}
					meta, _ := json.Marshal(job.Meta)
					payload := jobPayload{
						ID: job.ID, Kind: job.Kind, Title: job.Title,
						Lines: lines, Meta: meta,
					}
					raw, _ := json.Marshal(payload)
					rec.Job = raw
				}
			}
		}
		out = append(out, rec)
	}
	return out
}

func compareJob(t *testing.T, idx int, gotRaw, wantRaw json.RawMessage) {
	t.Helper()
	if len(wantRaw) == 0 {
		if len(gotRaw) != 0 {
			t.Errorf("[%d] unexpected job: %s", idx, gotRaw)
		}
		return
	}
	if len(gotRaw) == 0 {
		t.Errorf("[%d] missing job: %s", idx, wantRaw)
		return
	}
	var got, want jobPayload
	if err := json.Unmarshal(gotRaw, &got); err != nil {
		t.Fatalf("[%d] unmarshal got: %v", idx, err)
	}
	if err := json.Unmarshal(wantRaw, &want); err != nil {
		t.Fatalf("[%d] unmarshal want: %v", idx, err)
	}
	if got.ID != want.ID || got.Kind != want.Kind || got.Title != want.Title {
		t.Errorf("[%d] job header: got {%d %s %s}, want {%d %s %s}",
			idx, got.ID, got.Kind, got.Title, want.ID, want.Kind, want.Title)
	}
	if len(got.Lines) != len(want.Lines) {
		t.Errorf("[%d] job lines: got %d, want %d", idx, len(got.Lines), len(want.Lines))
		return
	}
	for i := range want.Lines {
		if got.Lines[i] != want.Lines[i] {
			t.Errorf("[%d] paper line %d:\n got  %q\n want %q",
				idx, i, got.Lines[i], want.Lines[i])
		}
	}
	// Compare as JSON with insignificant whitespace removed: Python's
	// json.dumps separates pairs with ", " while Go does not, but the key
	// order (which is asserted by construction) must still match.
	if strings.ReplaceAll(string(got.Meta), " ", "") !=
		strings.ReplaceAll(string(want.Meta), " ", "") {
		t.Errorf("[%d] job meta: got %s, want %s", idx, got.Meta, want.Meta)
	}
}
