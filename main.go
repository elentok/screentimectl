package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		runDaemon()
	case "setup":
		if err := runSetup(); err != nil {
			fmt.Fprintf(os.Stderr, "setup: %v\n", err)
			os.Exit(1)
		}
	case "doctor":
		runDoctor()
	case "logs":
		runLogs()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: screentimectl <command>")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  run     Start the daemon")
	fmt.Fprintln(os.Stderr, "  setup   Install system dependencies (run as root)")
	fmt.Fprintln(os.Stderr, "  doctor  Check system configuration")
	fmt.Fprintln(os.Stderr, "  logs    Follow service logs")
}

func runDaemon() {
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	bot, err := newBot(cfg)
	if err != nil {
		log.Fatalf("telegram: %v", err)
	}

	go bot.run()
	go startHTTPServer(cfg, bot)

	log.Printf("screentimectl started (machine: %s)", cfg.MachineName)
	bot.sendAll(fmt.Sprintf("screentimectl started (machine: %s)", cfg.MachineName))

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down")
}

func runLogs() {
	cmd := exec.Command("journalctl", "-u", "screentimectl", "-f")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "journalctl: %v\n", err)
		os.Exit(1)
	}
}
