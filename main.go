package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"strings"
	"syscall"
	"time"
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
	case "status":
		runStatus()
	case "ask":
		runAsk()
	case "hours":
		runHours()
	case "say":
		runSay()
	case "check-login":
		os.Exit(runCheckLogin())
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: screentimectl <command>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Admin commands:")
	fmt.Fprintln(os.Stderr, "  run          Start the daemon")
	fmt.Fprintln(os.Stderr, "  setup        Install system dependencies (run as root)")
	fmt.Fprintln(os.Stderr, "  doctor       Check system configuration")
	fmt.Fprintln(os.Stderr, "  logs         Follow service logs")
	fmt.Fprintln(os.Stderr, "  hours        View or set allowed hours for a user")
	fmt.Fprintln(os.Stderr, "  say          Speak a message to a user via TTS")
	fmt.Fprintln(os.Stderr, "  check-login  Check if a user is allowed to log in (used by PAM)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "User commands:")
	fmt.Fprintln(os.Stderr, "  status       Show your remaining screen time")
	fmt.Fprintln(os.Stderr, "  ask          Request more screen time")
}

func runDaemon() {
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	store, err := NewUsageStore(usagePath())
	if err != nil {
		log.Fatalf("usage store: %v", err)
	}

	mgr := NewSessionManager(cfg, store, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start session manager and HTTP server immediately — they don't need Telegram
	go mgr.Run(ctx)
	go startHTTPServer(cfg, nil, mgr)

	// Connect to Telegram with retries
	go func() {
		var bot *Bot
		for {
			var err error
			bot, err = newBot(cfg, mgr)
			if err == nil {
				break
			}
			log.Printf("telegram: %v (retrying in 30s)", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
			}
		}
		mgr.bot = bot
		log.Printf("screentimectl started (machine: %s)", cfg.MachineName)
		bot.sendAll(fmt.Sprintf("screentimectl started (machine: %s)", cfg.MachineName))
		bot.run()
	}()

	log.Printf("screentimectl starting (machine: %s)", cfg.MachineName)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down")
	cancel()
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

const defaultAddr = "127.0.0.1:3847"

func runStatus() {
	username := currentUser()
	addr := daemonAddr()

	resp, err := http.Get(fmt.Sprintf("http://%s/status?user=%s", addr, username))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to daemon: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "error: %s\n", body)
		os.Exit(1)
	}

	var data map[string]any
	json.Unmarshal(body, &data)

	ut := UserTime{
		RemainingSeconds: int(data["remaining_seconds"].(float64)),
		UsedSeconds:      int(data["used_seconds"].(float64)),
	}

	fmt.Printf("You have %s remaining (used %s today)\n", ut.RemainingStr(), ut.UsedStr())
	if start, ok := data["allowed_hours_start"]; ok {
		end := data["allowed_hours_end"]
		fmt.Printf("Allowed hours: %dam - %dpm\n", int(start.(float64)), int(end.(float64))%12)
	}
}

func runAsk() {
	username := currentUser()
	addr := daemonAddr()

	minutes := "15"
	if len(os.Args) >= 3 {
		minutes = os.Args[2]
	}

	resp, err := http.Post(
		fmt.Sprintf("http://%s/request-more-time?user=%s&minutes=%s", addr, username, minutes),
		"", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to daemon: %v\n", err)
		os.Exit(1)
	}
	resp.Body.Close()

	if resp.StatusCode == 200 {
		fmt.Println("Request sent! Your parents have been notified.")
	} else {
		fmt.Fprintf(os.Stderr, "request failed (status %d)\n", resp.StatusCode)
		os.Exit(1)
	}
}

func daemonAddr() string {
	if addr := os.Getenv("SCREENTIMECTL_ADDR"); addr != "" {
		return addr
	}
	return defaultAddr
}

func runHours() {
	if len(os.Args) < 3 || len(os.Args) > 4 {
		fmt.Fprintln(os.Stderr, "Usage: screentimectl hours {user} [start-end]")
		os.Exit(1)
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	username := os.Args[2]
	u := cfg.getUser(username)
	if u == nil {
		fmt.Fprintf(os.Stderr, "unknown user: %s\n", username)
		os.Exit(1)
	}

	if len(os.Args) == 3 {
		fmt.Printf("Allowed hours for %s: %dam - %dpm\n",
			capitalize(username), u.AllowedHours.Start, u.AllowedHours.End%12)
		return
	}

	start, end, err := parseHoursRange(os.Args[3])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid hours: %v\n", err)
		os.Exit(1)
	}

	u.AllowedHours = AllowedHours{Start: start, End: end}
	if err := cfg.save(configPath); err != nil {
		fmt.Fprintf(os.Stderr, "failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Updated allowed hours for %s: %dam - %dpm\n",
		capitalize(username), start, end%12)
}

func runSay() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: screentimectl say {user} {text...}")
		os.Exit(1)
	}
	username := os.Args[2]
	text := strings.Join(os.Args[3:], " ")
	sendTTS(username, text)
}

func runCheckLogin() int {
	pamUser := os.Getenv("PAM_USER")
	if pamUser == "" {
		return 0 // not called from PAM, allow
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		return 0 // can't load config, don't block login
	}

	u := cfg.getUser(pamUser)
	if u == nil {
		return 0 // not a managed user, allow
	}

	// Check for override
	store, err := NewUsageStore(usagePath())
	if err != nil {
		return 0 // can't load usage, don't block
	}

	if store.HasOverride(pamUser) {
		return 0
	}

	// Check allowed hours
	if !isWithinAllowedHours(u.AllowedHours) {
		fmt.Fprintf(os.Stderr, "Login not allowed outside %d:00-%d:00\n",
			u.AllowedHours.Start, u.AllowedHours.End)
		return 1
	}

	// Check remaining time
	ut := store.GetUserTime(pamUser, u.DailyLimitMinutes*60)
	if ut.RemainingSeconds <= 0 {
		fmt.Fprintf(os.Stderr, "No screen time remaining today\n")
		return 1
	}

	return 0
}

func currentUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	u, err := user.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not determine current user: %v\n", err)
		os.Exit(1)
	}
	return u.Username
}

