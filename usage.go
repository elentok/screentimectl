package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const usageDir = "/var/lib/screentimectl"

type UsageStore struct {
	mu   sync.Mutex
	path string
	data UsageData
}

type UsageData struct {
	Date  string                `json:"date"`
	Users map[string]*UserUsage `json:"users"`
}

type UserUsage struct {
	UsedSeconds        int       `json:"used_seconds"`
	BonusSeconds       int       `json:"bonus_seconds"`
	OverrideUntil      time.Time `json:"override_until,omitempty"`
	NotifiedThresholds []int     `json:"notified_thresholds,omitempty"`
	ExpiryHandled      bool      `json:"expiry_handled,omitempty"`
}

func NewUsageStore(path string) (*UsageStore, error) {
	s := &UsageStore{path: path}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	s.resetIfNewDay()
	return s, nil
}

func (s *UsageStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		s.data = UsageData{
			Date:  today(),
			Users: make(map[string]*UserUsage),
		}
		return err
	}
	if err := json.Unmarshal(data, &s.data); err != nil {
		return fmt.Errorf("parsing usage data: %w", err)
	}
	if s.data.Users == nil {
		s.data.Users = make(map[string]*UserUsage)
	}
	return nil
}

func (s *UsageStore) save() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *UsageStore) resetIfNewDay() bool {
	if s.data.Date != today() {
		s.data = UsageData{
			Date:  today(),
			Users: make(map[string]*UserUsage),
		}
		return true
	}
	return false
}

func (s *UsageStore) getUser(name string) *UserUsage {
	u, ok := s.data.Users[name]
	if !ok {
		u = &UserUsage{}
		s.data.Users[name] = u
	}
	return u
}

func (s *UsageStore) AddUsedTime(user string, seconds int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getUser(user).UsedSeconds += seconds
}

func (s *UsageStore) AddBonusTime(user string, seconds int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.getUser(user)
	u.BonusSeconds += seconds
	u.ExpiryHandled = false
}

func (s *UsageStore) SetOverride(user string, until time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getUser(user).OverrideUntil = until
}

func (s *UsageStore) HasOverride(user string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.getUser(user)
	return !u.OverrideUntil.IsZero() && time.Now().Before(u.OverrideUntil)
}

func (s *UsageStore) GetUserTime(user string, limitSeconds int) UserTime {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.getUser(user)
	remaining := limitSeconds + u.BonusSeconds - u.UsedSeconds
	if remaining < 0 {
		remaining = 0
	}
	return UserTime{
		RemainingSeconds: remaining,
		UsedSeconds:      u.UsedSeconds,
	}
}

func (s *UsageStore) SetRemainingTime(user string, seconds int, limitSeconds int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.getUser(user)
	// remaining = limit + bonus - used => used = limit + bonus - remaining
	u.UsedSeconds = limitSeconds + u.BonusSeconds - seconds
	if u.UsedSeconds < 0 {
		u.UsedSeconds = 0
	}
	u.ExpiryHandled = seconds == 0
}

func (s *UsageStore) MarkNotified(user string, threshold int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.getUser(user)
	u.NotifiedThresholds = append(u.NotifiedThresholds, threshold)
}

func (s *UsageStore) AlreadyNotified(user string, threshold int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.getUser(user)
	for _, t := range u.NotifiedThresholds {
		if t == threshold {
			return true
		}
	}
	return false
}

func (s *UsageStore) ResetIfNewDay() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resetIfNewDay()
}

func (s *UsageStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save()
}

func (s *UsageStore) IsExpiryHandled(user string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getUser(user).ExpiryHandled
}

func (s *UsageStore) SetExpiryHandled(user string, handled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getUser(user).ExpiryHandled = handled
}

func usagePath() string {
	return filepath.Join(usageDir, "usage.json")
}

func today() string {
	return time.Now().Format("2006-01-02")
}
