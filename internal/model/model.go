package model

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
	"time"
)

// Form represents a form definition.
type Form struct {
	Handle    string    `json:"handle"`
	Workspace string    `json:"workspace"`
	Title     string    `json:"title"`
	Desc      string    `json:"desc,omitempty"`
	Fields    []Field   `json:"fields"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	SubCount  int       `json:"sub_count"`
}

// Field defines a single form field.
type Field struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Type     string `json:"type"` // text, email, number, textarea, select, checkbox
	Required bool   `json:"required"`
	Options  string `json:"options,omitempty"` // comma-separated for select type
}

// Submission represents a form submission.
type Submission struct {
	Handle     string    `json:"handle"`
	FormHandle string    `json:"form_handle"`
	Workspace  string    `json:"workspace"`
	Data       string    `json:"data"` // key=value pairs, one per line
	CreatedAt  time.Time `json:"created_at"`
}

// Workspace represents a tenant workspace.
type Workspace struct {
	Handle    string    `json:"handle"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Plan      string    `json:"plan"`
	CreatedAt time.Time `json:"created_at"`
}

// Token represents an auth token.
type Token struct {
	Token     string    `json:"token"`
	Workspace string    `json:"workspace"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// OTP represents a one-time password.
type OTP struct {
	Email     string    `json:"email"`
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

// genHandle generates a short handle like form_a1b2c.
func genHandle(prefix string) string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	s := base32.StdEncoding.EncodeToString(b)
	s = strings.ToLower(strings.TrimRight(s, "="))
	return fmt.Sprintf("%s_%s", prefix, s[:5])
}

// NewFormHandle generates a form handle.
func NewFormHandle() string {
	return genHandle("form")
}

// NewSubmissionHandle generates a submission handle.
func NewSubmissionHandle() string {
	return genHandle("sub")
}

// NewWorkspaceHandle generates a workspace handle.
func NewWorkspaceHandle() string {
	return genHandle("ws")
}

// ValidateField checks that a field has required attributes.
func ValidateField(f Field) error {
	if f.Name == "" {
		return fmt.Errorf("field name is required")
	}
	if f.Label == "" {
		return fmt.Errorf("field label is required")
	}
	switch f.Type {
	case "text", "email", "number", "textarea", "select", "checkbox":
	default:
		return fmt.Errorf("field type must be one of: text, email, number, textarea, select, checkbox")
	}
	if f.Type == "select" && f.Options == "" {
		return fmt.Errorf("select field requires options (comma-separated)")
	}
	return nil
}
