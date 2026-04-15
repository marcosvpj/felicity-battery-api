package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// serverState holds the latest poll result shared between the poller goroutine
// and the HTTP handlers. All access is protected by mu.
type serverState struct {
	mu       sync.RWMutex
	latest   *HistoryRecord
	lastErr  error
	lastPoll time.Time
}

func (s *serverState) updateLatest(rec HistoryRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latest = &rec
	s.lastErr = nil
	s.lastPoll = time.Now()
}

func (s *serverState) updateError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastErr = err
	s.lastPoll = time.Now()
}

// snapshot returns copies of the latest record and metadata under RLock.
func (s *serverState) snapshot() (rec *HistoryRecord, lastErr error, lastPoll time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.latest != nil {
		cp := *s.latest
		rec = &cp
	}
	return rec, s.lastErr, s.lastPoll
}

// runPoller fetches battery data every interval, writing results to historyPath
// and updating state. It fires immediately on start, then ticks every interval.
func runPoller(c *client, sn, historyPath string, state *serverState, interval time.Duration) {
	poll := func() {
		snap, err := c.getSnapshot(sn)
		if err != nil {
			log.Printf("[poller] error: %v", err)
			state.updateError(err)
			return
		}
		if historyPath != "" {
			if err := AppendHistory(historyPath, snap); err != nil {
				log.Printf("[poller] history write error: %v", err)
			}
		}
		rec := snapshotToRecord(snap)
		state.updateLatest(rec)
		log.Printf("[poller] ok: SOC=%.0f%% Volt=%.2fV Curr=%.2fA", rec.SOC, rec.VoltV, rec.CurrA)
	}

	poll()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		poll()
	}
}

// StartServer registers routes and starts the HTTP server. This call blocks.
func StartServer(addr string, state *serverState, historyPath string) error {
	mux := http.NewServeMux()
	mux.Handle("/", handleDashboard())
	mux.Handle("/api/status", handleStatus(state))
	mux.Handle("/api/history", handleHistory(historyPath))
	mux.Handle("/api/health", handleHealth(state))

	srv := &http.Server{
		Addr:         addr,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return srv.ListenAndServe()
}

// corsMiddleware adds CORS headers to every response and short-circuits OPTIONS.
func corsMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// handleStatus returns the latest HistoryRecord as JSON, or 503 if no poll has
// succeeded yet.
func handleStatus(state *serverState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec, lastErr, _ := state.snapshot()
		if rec == nil {
			msg := "no data yet"
			if lastErr != nil {
				msg = fmt.Sprintf("no data yet: %v", lastErr)
			}
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": msg})
			return
		}
		writeJSON(w, http.StatusOK, rec)
	}
}

// handleHistory reads the JSONL history file, applies optional from/to/limit/offset
// filters, and returns results newest-first.
func handleHistory(historyPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		var from, to time.Time
		if s := q.Get("from"); s != "" {
			t, err := time.Parse(time.RFC3339, s)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid 'from': use RFC3339"})
				return
			}
			from = t
		}
		if s := q.Get("to"); s != "" {
			t, err := time.Parse(time.RFC3339, s)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid 'to': use RFC3339"})
				return
			}
			to = t
		}

		limit := 500
		if s := q.Get("limit"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil || n < 1 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid 'limit'"})
				return
			}
			if n > 10000 {
				n = 10000
			}
			limit = n
		}

		offset := 0
		if s := q.Get("offset"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil || n < 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid 'offset'"})
				return
			}
			offset = n
		}

		records, err := readHistory(historyPath, from, to, limit, offset)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if records == nil {
			records = []HistoryRecord{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"count":   len(records),
			"records": records,
		})
	}
}

// handleHealth returns a health object including whether a recent poll succeeded.
func handleHealth(state *serverState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, lastErr, lastPoll := state.snapshot()

		stale := lastPoll.IsZero() || time.Since(lastPoll) > 10*time.Minute
		ok := lastErr == nil && !stale

		var lastPollStr *string
		if !lastPoll.IsZero() {
			s := lastPoll.UTC().Format(time.RFC3339)
			lastPollStr = &s
		}
		var lastErrStr *string
		if lastErr != nil {
			s := lastErr.Error()
			lastErrStr = &s
		}

		status := http.StatusOK
		if !ok {
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, map[string]any{
			"ok":         ok,
			"last_poll":  lastPollStr,
			"last_error": lastErrStr,
			"data_stale": stale,
		})
	}
}

// writeJSON marshals v as JSON and writes it with Content-Type application/json.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[http] encode error: %v", err)
	}
}
