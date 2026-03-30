package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

func startHTTPServer(cfg *Config, bot *Bot, mgr *SessionManager) {
	mux := http.NewServeMux()
	mux.HandleFunc("/request-more-time", func(w http.ResponseWriter, r *http.Request) {
		handleRequestMoreTime(w, r, cfg, bot, mgr)
	})
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		handleStatus(w, r, cfg, mgr)
	})

	log.Printf("http: listening on %s", cfg.Server.ListenAddr)
	if err := http.ListenAndServe(cfg.Server.ListenAddr, mux); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

func handleRequestMoreTime(w http.ResponseWriter, r *http.Request, cfg *Config, bot *Bot, mgr *SessionManager) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := r.URL.Query().Get("user")
	if user == "" {
		http.Error(w, "missing user parameter", http.StatusBadRequest)
		return
	}

	if !cfg.isValidUser(user) {
		http.Error(w, fmt.Sprintf("unknown user: %s", user), http.StatusBadRequest)
		return
	}

	minutesStr := r.URL.Query().Get("minutes")
	var suggestedMinutes int
	if minutesStr != "" {
		var err error
		suggestedMinutes, err = strconv.Atoi(minutesStr)
		if err != nil || suggestedMinutes <= 0 {
			http.Error(w, "invalid minutes parameter", http.StatusBadRequest)
			return
		}
	}

	ut, err := mgr.GetUserTime(user)
	if err != nil {
		log.Printf("http: GetUserTime(%s): %v", user, err)
		http.Error(w, "failed to get user time", http.StatusInternalServerError)
		return
	}

	var text string
	name := capitalize(user)
	if suggestedMinutes > 0 {
		text = fmt.Sprintf("%s has already used the computer for %s. %s is asking for more time (suggested: %dm)",
			name, ut.UsedStr(), name, suggestedMinutes)
	} else {
		text = fmt.Sprintf("%s has already used the computer for %s. %s is asking for more time",
			name, ut.UsedStr(), name)
	}

	bot.sendAll(text)
	w.WriteHeader(http.StatusOK)
}

func handleStatus(w http.ResponseWriter, r *http.Request, cfg *Config, mgr *SessionManager) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := r.URL.Query().Get("user")
	if user == "" {
		http.Error(w, "missing user parameter", http.StatusBadRequest)
		return
	}

	ut, err := mgr.GetUserTime(user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := map[string]any{
		"remaining_seconds": ut.RemainingSeconds,
		"used_seconds":      ut.UsedSeconds,
	}
	if u := cfg.getUser(user); u != nil {
		resp["allowed_hours_start"] = u.AllowedHours.Start
		resp["allowed_hours_end"] = u.AllowedHours.End
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
