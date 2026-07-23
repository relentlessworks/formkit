package api

import (
	"net/http"
	"strings"
)

// authMiddleware checks for a valid bearer token.
func (h *Handler) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			writeError(w, r, http.StatusUnauthorized, "missing auth token", "call POST /auth/request with email to get an OTP, then POST /auth/verify to get a bearer token")
			return
		}
		t, err := h.auth.ValidateToken(token)
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, "invalid or expired token", "request a new token via POST /auth/request then POST /auth/verify")
			return
		}
		r.Header.Set("X-Workspace", t.Workspace)
		r.Header.Set("X-Email", t.Email)
		next(w, r)
	}
}

// extractToken gets the bearer token from the Authorization header.
func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// getWorkspace extracts the workspace from the request context (set by authMiddleware).
func getWorkspace(r *http.Request) string {
	return r.Header.Get("X-Workspace")
}
