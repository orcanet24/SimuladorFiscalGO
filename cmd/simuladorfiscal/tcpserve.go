// Command simuladorfiscal serve: TCP listener that speaks the printer
// protocol, acting as a virtual fiscal printer for POS/ERP testing without
// com0com or physical hardware.
//
//	simuladorfiscal serve --model tfhka --addr 127.0.0.1:2011
package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"simuladorfiscalgo/internal/simulator"
)

// runServe starts the TCP virtual-port listener.
func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	model := fs.String("model", "tfhka", "printer brand: tfhka|hasar|bixolon|epson")
	addr := fs.String("addr", "127.0.0.1:2011", "TCP listen address (virtual COM)")
	dataDir := fs.String("data-dir", "", "directory for persisted state")
	logFile := fs.String("log", "fiscalsim.log", "log file path")
	_ = fs.Parse(args)

	statePath := ""
	if *dataDir != "" {
		if err := os.MkdirAll(*dataDir, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		statePath = filepath.Join(*dataDir, "state_"+*model+".json")
	}

	sim, err := simulator.CreateSimulator(*model, "", statePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	log := openLog(*logFile)

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error escuchando en", *addr, ":", err)
		os.Exit(1)
	}
	fmt.Printf("============================================================================\n")
	fmt.Printf("  Impresora fiscal virtual %s %s escuchando en tcp://%s\n", sim.Brand, sim.Model, *addr)
	fmt.Printf("  Conecte el ERP/POS con:  --port tcp:%s\n", *addr)
	fmt.Printf("  Ctrl+C para detener.\n")
	fmt.Printf("============================================================================\n\n")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		fmt.Println("\nDeteniendo impresora virtual...")
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			// Listener closed: normal shutdown.
			return
		}
		fmt.Printf("[%s] cliente conectado: %s\n", sim.Brand, conn.RemoteAddr())
		log("CONNECT: " + conn.RemoteAddr().String())
		go handleServeConn(sim, conn, log)
	}
}

// handleServeConn serves a single TCP client: newline-terminated commands in,
// responses with CRLF out (same framing as the physical serial protocol).
func handleServeConn(sim *simulator.Simulator, conn net.Conn, log func(string)) {
	defer func() {
		fmt.Printf("[%s] cliente desconectado: %s\n", sim.Brand, conn.RemoteAddr())
		log("DISCONNECT: " + conn.RemoteAddr().String())
		_ = conn.Close()
	}()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 4096), 64*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		response := sim.ProcessCommand(line)
		log("CMD: " + line + " -> " + response)
		if _, err := conn.Write([]byte(response + "\r\n")); err != nil {
			return
		}
	}
}
