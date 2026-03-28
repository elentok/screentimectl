package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
)

func startHTTPServer(cfg *Config, bot *Bot) {
	mux := http.NewServeMux()
	mux.HandleFunc("/request-more-time", func(w http.ResponseWriter, r *http.Request) {
		handleRequestMoreTime(w, r, cfg, bot)
	})

	log.Printf("http: listening on %s", cfg.Server.ListenAddr)
	if err := http.ListenAndServe(cfg.Server.ListenAddr, mux); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

func handleRequestMoreTime(w http.ResponseWriter, r *http.Request, cfg *Config, bot *Bot) {
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

	ut, err := GetUserTime(user)
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
