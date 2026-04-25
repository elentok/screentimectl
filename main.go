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
	case "give":
		runAdminCommand("give", os.Args[2:])
	case "lock":
		runAdminCommand("lock", os.Args[2:])
	case "unlock":
		runAdminCommand("unlock", os.Args[2:])
	case "hours":
		runAdminCommand("hours", os.Args[2:])
	case "say":
		runAdminCommand("say", os.Args[2:])
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
	fmt.Fprintln(os.Stderr, "  give         Add time for a user")
	fmt.Fprintln(os.Stderr, "  lock         Lock a user's screen and account")
	fmt.Fprintln(os.Stderr, "  unlock       Set remaining time and allow login")
	fmt.Fprintln(os.Stderr, "  status       Show remaining time for the current or target user")
	fmt.Fprintln(os.Stderr, "  hours        View or set allowed hours for a user")
	fmt.Fprintln(os.Stderr, "  say          Send a spoken and desktop message to a user")
	fmt.Fprintln(os.Stderr, "  check-login  Check if a user is allowed to log in (used by PAM)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "User commands:")
	fmt.Fprintln(os.Stderr, "  status       Show your remaining screen time")
	fmt.Fprintln(os.Stderr, "  status --compact")
	fmt.Fprintln(os.Stderr, "               Show only remaining screen time")
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

	actLog := NewActivityLog(logDir)
	mgr := NewSessionManager(cfg, store, nil, actLog)
	mgr.startupUnlock()

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
	mgr.LogShutdown()
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
	compact, username, err := parseStatusArgs(os.Args[2:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Usage: screentimectl status [user] [--compact]: %v\n", err)
		os.Exit(1)
	}

	if username != "" {
		runAdminStatus(username, compact)
		return
	}

	current := currentUser()
	if commands, err := newAdminCommands(); err == nil && !commands.cfg.isValidUser(current) {
		if compact {
			username, err := commands.defaultUser()
			if err != nil {
				fmt.Fprintf(os.Stderr, "status: %v\n", err)
				os.Exit(1)
			}
			text, err := commands.StatusSummaryForUser(username, true)
			if err != nil {
				fmt.Fprintf(os.Stderr, "status: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(text)
			return
		}

		text, err := commands.Status(nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "status: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(text)
		return
	}

	printStatusFromHTTP(current, compact)
}

func parseStatusArgs(args []string) (bool, string, error) {
	var compact bool
	var username string
	for _, arg := range args {
		if arg == "--compact" {
			compact = true
			continue
		}
		if username != "" {
			return false, "", fmt.Errorf("too many arguments")
		}
		username = arg
	}
	return compact, username, nil
}

func runAdminStatus(username string, compact bool) {
	commands, err := newAdminCommands()
	if err != nil {
		fmt.Fprintf(os.Stderr, "status: %v\n", err)
		os.Exit(1)
	}
	if compact {
		text, err := commands.StatusSummaryForUser(username, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "status: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(text)
		return
	}
	text, err := commands.StatusForUser(username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "status: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(text)
}

func printStatusFromHTTP(username string, compact bool) {
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

	if compact {
		fmt.Println(formatStatusSummary(ut, true))
		return
	}

	fmt.Println(formatStatusSummary(ut, false))
	if start, ok := data["allowed_hours_start"]; ok {
		end := data["allowed_hours_end"]
		startMin, _ := data["allowed_hours_start_minute"].(float64)
		endMin, _ := data["allowed_hours_end_minute"].(float64)
		fmt.Printf("Allowed hours: %s - %s\n",
			formatHour(int(start.(float64)), int(startMin)),
			formatHour(int(end.(float64)), int(endMin)))
	}
	if status, ok := data["session_status"]; ok {
		fmt.Printf("Session: %s\n", status)
	}

	if activity, ok := data["activity"]; ok && activity != nil {
		if entries, ok := activity.([]any); ok && len(entries) > 0 {
			var logEntries []LogEntry
			for _, e := range entries {
				if m, ok := e.(map[string]any); ok {
					logEntries = append(logEntries, LogEntry{
						Time:   fmt.Sprint(m["time"]),
						Status: fmt.Sprint(m["status"]),
					})
				}
			}
			if len(logEntries) > 0 {
				fmt.Printf("\nToday:\n%s", FormatTimeline(logEntries))
			}
		}
	}
}

func newAdminCommands() (*AdminCommands, error) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	store, err := NewUsageStore(usagePath())
	if err != nil {
		return nil, fmt.Errorf("usage store: %w", err)
	}

	return NewAdminCommands(cfg, NewSessionManager(cfg, store, nil, NewActivityLog(logDir))), nil
}

func runAdminCommand(command string, args []string) {
	commands, err := newAdminCommands()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", command, err)
		os.Exit(1)
	}

	var text string
	switch command {
	case "give":
		text, err = commands.Give(args)
	case "lock":
		text, err = commands.Lock(args)
	case "unlock":
		text, err = commands.Unlock(args)
	case "hours":
		text, err = commands.Hours(args)
	case "say":
		text, err = commands.Say(args)
	default:
		err = fmt.Errorf("unknown admin command: %s", command)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", command, err)
		os.Exit(1)
	}
	fmt.Println(text)
}

func formatStatusSummary(ut UserTime, compact bool) string {
	if compact {
		return fmt.Sprintf("%s remaining", ut.RemainingStr())
	}
	return fmt.Sprintf("You have %s remaining (used %s today)", ut.RemainingStr(), ut.UsedStr())
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
		fmt.Printf("Login allowed only between %s and %s.\n",
			formatHour(u.AllowedHours.Start, u.AllowedHours.StartMinute),
			formatHour(u.AllowedHours.End, u.AllowedHours.EndMinute))
		return 1
	}

	// Check remaining time
	ut := store.GetUserTime(pamUser, u.DailyLimitMinutes*60)
	if ut.RemainingSeconds <= 0 {
		fmt.Println("No screen time remaining today. Ask a parent for more time.")
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
