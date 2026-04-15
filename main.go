package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

func defaultHistoryPath() string {
	return filepath.Join("data", "battery.jsonl")
}

func main() {
	user := flag.String("user", os.Getenv("FELICITY_USER"), "Account email (or set FELICITY_USER)")
	pass := flag.String("pass", os.Getenv("FELICITY_PASS"), "Account password (or set FELICITY_PASS)")
	sn := flag.String("device", "", "Device serial number (see Felicity app or API response)")
	watch := flag.Bool("watch", false, "Keep running and refresh every 5 minutes (CLI mode)")
	load := flag.Float64("load", 0, "Fixed load in watts for projection (e.g. -load 195)")
	historyFile := flag.String("history", defaultHistoryPath(), "Path to JSONL history file; empty string disables recording")
	serve := flag.String("serve", "", "Start HTTP API server on this address (e.g. -serve :8080)")
	flag.Parse()

	if *user == "" || *pass == "" {
		fmt.Fprintln(os.Stderr, "Usage: felicity-battery -user EMAIL -pass PASSWORD [-device SN] [-watch] [-history FILE] [-serve :8080]")
		fmt.Fprintln(os.Stderr, "       Credentials can also be set via FELICITY_USER / FELICITY_PASS env vars.")
		os.Exit(1)
	}

	c := newClient(*user, *pass)

	// Server mode: background poller + HTTP API. Blocks until the server exits.
	if *serve != "" {
		state := &serverState{}
		go runPoller(c, *sn, *historyFile, state, 5*time.Minute)
		log.Printf("[server] HTTP API listening on %s", *serve)
		if err := StartServer(*serve, state, *historyFile); err != nil {
			log.Fatal(err)
		}
		return
	}

	// CLI mode: single fetch or periodic watch loop.
	fetch := func() {
		snap, err := c.getSnapshot(*sn)
		if err != nil {
			log.Printf("error: %v", err)
			return
		}
		printBattery(snap, *load)
		if *historyFile != "" {
			if err := AppendHistory(*historyFile, snap); err != nil {
				log.Printf("history write error: %v", err)
			}
		}
	}

	fetch()

	if *watch {
		for range time.Tick(5 * time.Minute) {
			fetch()
		}
	}
}
