// Command simuladorfiscal is the single binary for the fiscal printer
// simulator: web panel, serial listener and interactive console.
//
//	simuladorfiscal                          # panel on http://127.0.0.1:8080
//	simuladorfiscal panel --port 9000
//	simuladorfiscal sim --model tfhka --port COM5
//	simuladorfiscal sim --model hasar --mode test
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"go.bug.st/serial"

	"simuladorfiscalgo/internal/protocol"
	"simuladorfiscalgo/internal/simulator"
	"simuladorfiscalgo/internal/web"
)

const version = "1.0.0"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"panel"}
	}

	switch args[0] {
	case "panel":
		runPanel(args[1:])
	case "sim":
		runSim(args[1:])
	case "serve":
		runServe(args[1:])
	case "version", "--version", "-v":
		fmt.Println("simuladorfiscal", version)
	case "help", "--help", "-h":
		usage()
	default:
		// Unknown verb: treat everything as panel flags for convenience.
		runPanel(args)
	}
}

func usage() {
	fmt.Print(`Simulador de impresoras fiscales (TFHKA, Hasar, Bixolon, Epson)

Uso:
  simuladorfiscal [panel] [flags]      Panel web (por defecto)
  simuladorfiscal sim [flags]          CLI serie / consola interactiva
  simuladorfiscal serve [flags]        Impresora fiscal virtual por TCP (COM virtual)
  simuladorfiscal version              Versión

Flags del panel:
  --host 127.0.0.1     Dirección de enlace
  --port 8080          Puerto HTTP
  --data-dir ./data    Estado persistido por marca
  --no-browser         No abrir el navegador
  --quiet              Menos salida por consola

Flags del CLI serie:
  --model tfhka        Marca: tfhka | hasar | bixolon | epson
  --port COM5          Puerto serie
  --baud 9600          Velocidad
  --mode fiscal        Modo inicial: fiscal | test
  --log fiscalsim.log  Archivo de log

Flags de la impresora virtual TCP:
  --model tfhka        Marca: tfhka | hasar | bixolon | epson
  --addr 127.0.0.1:2011 Dirección de escucha TCP (COM virtual)
  --data-dir ./data    Estado persistido por marca
  --log fiscalsim.log  Archivo de log
`)
}

// --------------------------------------------------------------- panel mode

func runPanel(args []string) {
	fs := flag.NewFlagSet("panel", flag.ExitOnError)
	host := fs.String("host", "127.0.0.1", "bind address")
	port := fs.Int("port", 8080, "HTTP port")
	dataDir := fs.String("data-dir", "", "directory for persisted state")
	noBrowser := fs.Bool("no-browser", false, "do not open the browser")
	quiet := fs.Bool("quiet", false, "less console output")
	_ = fs.Parse(args)

	if *dataDir == "" {
		*dataDir = "data"
	}

	panel, err := web.NewPanel(*dataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	addr := fmt.Sprintf("%s:%d", *host, *port)
	url := "http://" + addr

	if !*quiet {
		fmt.Printf("Panel fiscal corriendo en %s\n", url)
	}

	if !*noBrowser && (*host == "127.0.0.1" || *host == "localhost") {
		go func() {
			time.Sleep(time.Second)
			openBrowser(url)
		}()
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: web.Handler(panel),
	}

	// Ctrl+C shuts the listener down cleanly.
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		if !*quiet {
			fmt.Println("\nDeteniendo panel...")
		}
		_ = srv.Close()
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// openBrowser launches the platform browser without extra dependencies.
func openBrowser(url string) {
	var name string
	var args []string
	switch runtime.GOOS {
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		name, args = "open", []string{url}
	default:
		name, args = "xdg-open", []string{url}
	}
	_ = exec.Command(name, args...).Start()
}

// ----------------------------------------------------------------- sim mode

func runSim(args []string) {
	fs := flag.NewFlagSet("sim", flag.ExitOnError)
	model := fs.String("model", "tfhka", "printer brand: tfhka|hasar|bixolon|epson")
	portName := fs.String("port", "COM5", "serial port")
	baud := fs.Int("baud", 9600, "baud rate")
	mode := fs.String("mode", "fiscal", "initial mode: fiscal|test")
	logFile := fs.String("log", "fiscalsim.log", "log file path")
	_ = fs.Parse(args)

	sim, err := simulator.CreateSimulator(*model, "", "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	sim.State.Mode = *mode

	logger := openLog(*logFile)

	// Try a real serial port; fall back to interactive stdin/stdout.
	serial, serr := openSerial(*portName, *baud)
	if serr != nil {
		fmt.Printf("No se pudo abrir %s (%v). Modo interactivo.\n", *portName, serr)
		logger("modo interactivo: puerto serie no disponible")
	}

	fmt.Printf("============================================================\n")
	fmt.Printf("  Fiscal Printer Simulator - %s %s\n", sim.Brand, sim.Model)
	if serial != nil {
		fmt.Printf("  Modo: Serial (%s @ %d)\n", *portName, *baud)
	} else {
		fmt.Printf("  Modo: Interactive (sin puerto serie)\n")
	}
	fmt.Printf("  Type 'help' for examples, 'quit' to exit.\n")
	fmt.Printf("============================================================\n\n")

	if serial != nil {
		defer serial.Close()
		serialLoop(sim, serial, logger)
	} else {
		interactiveLoop(sim, logger)
	}

	fmt.Printf("\n%s Simulator stopped.\n", sim.Brand)
}

// serialLoop reads newline-terminated commands and writes CRLF responses.
func serialLoop(sim *simulator.Simulator, port serialPort, logger func(string)) {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, err := port.Read(tmp)
		if err != nil {
			return
		}
		buf = append(buf, tmp[:n]...)
		for {
			var idx, sep int = -1, 1
			if i := indexAny(buf, '\n'); i >= 0 {
				idx, sep = i, 1
			} else if i := indexAny(buf, '\r'); i >= 0 {
				idx, sep = i, 1
			}
			if idx < 0 {
				break
			}
			line := strings.TrimSpace(string(buf[:idx]))
			buf = buf[idx+sep:]
			if line == "" {
				continue
			}
			response := sim.ProcessCommand(line)
			logger("CMD: " + line + " -> " + response)
			_, _ = port.Write([]byte(response + "\r\n"))
		}
	}
}

// interactiveLoop is the stdin/stdout REPL.
func interactiveLoop(sim *simulator.Simulator, logger func(string)) {
	in := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("%s> ", sim.Brand)
		if !in.Scan() {
			return
		}
		line := strings.TrimSpace(in.Text())
		if line == "" {
			continue
		}
		switch strings.ToLower(line) {
		case "quit", "exit":
			return
		case "help":
			printHelp()
			continue
		case "status":
			printStatus(sim)
			continue
		}
		response := sim.ProcessCommand(line)
		fmt.Printf("  <- %s\n", response)
		logger("CMD: " + line + " -> " + response)
	}
}

func printHelp() {
	fmt.Print(`
--- TFHKA Commands ---
  !   001000 000001500 CAFE     (item general 16%)
  "   002000 000000800 PAN      (item reduced 8%)
  #   001000 000003000 LICORES  (item additional 30%)
  3                             (subtotal with print)
  4                             (subtotal silent)
  101                           (close and totalize)
  0                             (open cash drawer)
  S1                            (general status)
  U0X                           (X report)
  U0Z                           (Z report)

--- Hasar IxBatch Commands ---
  @OpenFiscalReceipt
  @PrintLine|Cafe|2|15.00|general
  @AddPayment|30.00|cash
  @Close
  @PrintXReport
  @CustomerData|J-12345678-9|EMPRESA C.A.|Av Principal

--- Common ---
  help      (show this help)
  status    (show printer status)
  quit      (exit)
`)
}

func printStatus(sim *simulator.Simulator) {
	st := sim.State
	status := st.GetStatusCode()
	errCode := st.GetErrorCode()

	fmt.Printf("\n--- %s %s Status ---\n", sim.Brand, sim.Model)
	fmt.Printf("  Serial:        %s\n", st.SerialNumber)
	fmt.Printf("  RIF:           %s\n", st.Rif)
	fmt.Printf("  Status:        0x%02X (%s)\n", status, protocol.STATUS_CODES[status])
	fmt.Printf("  Error:         0x%02X (%s)\n", errCode, protocol.ERROR_CODES[errCode])
	fmt.Printf("  Mode:          %s\n", st.Mode)
	fmt.Printf("  Doc open:      %t\n", st.DocumentOpen)
	fmt.Printf("  Doc type:      %s\n", st.DocumentType)
	fmt.Printf("  Invoices:      %d\n", st.InvoiceCounter)
	fmt.Printf("  Credit Notes:  %d\n", st.CreditNoteCounter)
	fmt.Printf("  Debit Notes:   %d\n", st.DebitNoteCounter)
	fmt.Printf("  Non-Fiscal:    %d\n", st.NonFiscalCounter)
	fmt.Printf("  Z Reports:     %d\n", st.ZReportCounter)
	fmt.Printf("  Total Sales:   %.2f\n", st.TotalSales)
	fmt.Printf("  Total Tax:     %.2f\n", st.TotalTax)
	fmt.Printf("  Audit Mem:     %.2f/%.2f MB\n", st.AuditMemoryFree, st.AuditMemoryTotal)
	fmt.Printf("  Commands:      %d\n\n", sim.CommandCount)
}

func openLog(path string) func(string) {
	if path == "" {
		return func(string) {}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return func(string) {}
	}
	return func(msg string) {
		_, _ = fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), msg)
	}
}

func indexAny(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}

// serialPort is the transport used by the serial loop (real UART or any
// io.ReadWriteCloser for tests).
type serialPort interface {
	io.ReadWriteCloser
}

// openSerial opens a real serial port at the given baud rate.
func openSerial(name string, baud int) (serialPort, error) {
	p, err := serial.Open(name, &serial.Mode{BaudRate: baud})
	if err != nil {
		return nil, err
	}
	return p, nil
}
