package simulator

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var tsRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`)

// normalizeTS masks timestamps the same way gen_golden.py does.
func normalizeTS(s string) string {
	return tsRe.ReplaceAllString(s, "<TS>")
}

type goldenRecord struct {
	Scenario string          `json:"scenario"`
	I        int             `json:"i"`
	Command  string          `json:"command"`
	Response string          `json:"response"`
	Job      json.RawMessage `json:"job"`
}

func goldenDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "testdata", "golden")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("golden dir missing: %v", err)
	}
	return dir
}

func readGoldenJSONL(t *testing.T, name string) []goldenRecord {
	t.Helper()
	f, err := os.Open(filepath.Join(goldenDir(t), name))
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer f.Close()

	var out []goldenRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec goldenRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("bad jsonl line: %v", err)
		}
		out = append(out, rec)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// corpus mirrors tools/golden/gen_golden.py exactly: one fresh simulator per
// scenario, plus a warm-up S1 command before the recorded commands.
var tfhkaCorpus = []scenario{
	{"invoice_full", []string{
		"iR*J-12345678-9", "iS*EMPRESA DE PRUEBA C.A.",
		"i01Av. Principal, Caracas",
		"!   2.000     50.00HARINA 1KG MAIZ",
		`"   3.000     10.50PAN TAJADO`,
		"#   1.000    100.00LICOR PREMIUM",
		"    1.000      5.00ARTICULO EXENTO",
		"3", "1001031.00", "101", "S1", "U0X", "U0Z",
	}},
	{"payments_discounts", []string{
		"!   1.000    100.00SERVICIO",
		"70010.00", "7015.00", "103Pago parcial", "10050.00",
		"S4", "S2", "101",
	}},
	{"payment_methods", []string{
		"!   1.000     45.70PRODUCTO",
		"3", "103card", "10045.70", "101",
	}},
	{"credit_note", []string{
		"iR*J-12345678-9", "iS*EMPRESA C.A.",
		"!   1.000     15.00CAFE", "10015.00", "101",
		"d1", "iR*J-12345678-9", "iS*EMPRESA C.A.",
		"iF*001", "iD*2024-01-15", "iI*MLTFHKA001",
		"d1   1.000     15.00CAFE", "101",
	}},
	{"debit_note", []string{
		"!   1.000     20.00TE", "10020.00", "101",
		"d2", "iF*001", "`1   1.000     20.00TE", "101",
	}},
	{"non_fiscal", []string{
		"80$", "80!PRESUPUESTO", "80*Linea de texto libre", "81",
	}},
	{"status_matrix", []string{
		"S1", "S2", "S3", "S4", "S5", "S8E", "S8P",
		"0", "3", "4", "@COMENTARIO prueba",
	}},
	{"errors", []string{"", " basura ", "ZZZZ", "d", "100", "iR*"}},
}

type scenario struct {
	name     string
	commands []string
}

var ixbatchCorpus = []scenario{
	{"hasar", []string{
		"@Status",
		"@OpenFiscalReceipt",
		"@CustomerData|J-12345678-9|EMPRESA C.A.|Av Principal",
		"@PrintLine|CAFE|2|15.00|general",
		"@PrintLine|PAN|3|8.00|reduced",
		"@PrintLine|ARROZ|1|5.00|exento",
		"@Subtotal",
		"@AddPayment|54.00|cash",
		"@Close",
		"@PrintXReport",
		"@PrintZReport",
		"@OpenCreditNote", "@PrintLine|NC ITEM|1|10.00|general", "@Close",
		"@OpenDebitNote", "@PrintLine|ND ITEM|1|10.00|general", "@Close",
		"@OpenNonFiscalDoc", "@PrintNonFiscalText|nota", "@CloseNonFiscalDoc",
		"@OpenDrawer",
		"@PrintLineItem|OTRO|1|10.00|1|M",
		"@UnknownCommand|x",
		"",
	}},
}

func TestCommandsGolden(t *testing.T) {
	cases := []struct {
		file   string
		brand  string
		corpus []scenario
	}{
		{"commands_tfhka.jsonl", "tfhka", tfhkaCorpus},
		{"commands_hasar.jsonl", "hasar", ixbatchCorpus},
		{"commands_bixolon.jsonl", "bixolon", ixbatchCorpus},
		{"commands_epson.jsonl", "epson", ixbatchCorpus},
	}
	for _, c := range cases {
		t.Run(c.brand, func(t *testing.T) {
			want := readGoldenJSONL(t, c.file)
			got := replay(c.brand, c.corpus)
			if len(got) != len(want) {
				t.Fatalf("record count: got %d, want %d", len(got), len(want))
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
			}
		})
	}
}

// replay runs each scenario against a fresh simulator, mirroring gen_golden.
func replay(brand string, corpus []scenario) []goldenRecord {
	var out []goldenRecord
	for _, sc := range corpus {
		sim, err := CreateSimulator(brand, "", "")
		if err != nil {
			panic(err)
		}
		sim.ProcessCommand("S1") // warm-up parity with the Python harness
		for i, cmd := range sc.commands {
			resp := sim.ProcessCommand(cmd)
			out = append(out, goldenRecord{
				Scenario: sc.name, I: i, Command: cmd,
				Response: normalizeTS(resp),
			})
		}
	}
	return out
}
