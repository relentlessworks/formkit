package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// wantsJSON checks if the client wants JSON responses.
func wantsJSON(r *http.Request) bool {
	if r.URL.Query().Get("format") == "json" {
		return true
	}
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json")
}

// writeRecord writes a single record as plain text or JSON.
func writeRecord(w http.ResponseWriter, r *http.Request, status int, record string, obj interface{}) {
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(obj)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(record + "\n"))
}

// writeRecords writes multiple records as plain text or JSON.
func writeRecords(w http.ResponseWriter, r *http.Request, status int, records []string, obj interface{}) {
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(obj)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	for _, rec := range records {
		_, _ = w.Write([]byte(rec + "\n"))
	}
}

// writeError writes an error response with a hint.
func writeError(w http.ResponseWriter, r *http.Request, status int, msg, hint string) {
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": msg,
			"hint":  hint,
		})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte("error: " + msg + " | hint: " + hint + "\n"))
}

// writeOK writes a simple success message.
func writeOK(w http.ResponseWriter, r *http.Request, msg string) {
	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": msg})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok: " + msg + "\n"))
}
