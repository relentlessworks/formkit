package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/relentlessworks/formkit/internal/model"
	"github.com/relentlessworks/formkit/internal/store"
)

// Auth handles OTP and token management.
type Auth struct {
	store *store.Store
	smtp  string
}

// New creates a new auth manager.
func New(s *store.Store, smtp string) *Auth {
	return &Auth{store: s, smtp: smtp}
}

// RequestOTP generates and stores an OTP for the given email.
// If no SMTP is configured, the OTP is logged to stderr.
func (a *Auth) RequestOTP(email string) (string, error) {
	code := genOTPCode()
	otp := &model.OTP{
		Email:     email,
		Code:      code,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	if err := a.store.SaveOTP(otp); err != nil {
		return "", err
	}

	if a.smtp == "" {
		fmt.Fprintf(os.Stderr, "[formkit] OTP for %s: %s\n", email, code)
	}

	return code, nil
}

// VerifyOTP validates the OTP and creates a workspace + token if needed.
func (a *Auth) VerifyOTP(email, code string) (*model.Token, error) {
	otp, err := a.store.GetOTP(email)
	if err != nil {
		return nil, fmt.Errorf("no OTP requested for this email | hint: call POST /auth/request with email first")
	}
	if time.Now().After(otp.ExpiresAt) {
		_ = a.store.DeleteOTP(email)
		return nil, fmt.Errorf("OTP expired | hint: request a new OTP via POST /auth/request")
	}
	if otp.Code != code {
		return nil, fmt.Errorf("invalid OTP code | hint: check the code and try again")
	}

	// Find or create workspace
	ws, err := a.store.GetWorkspaceByEmail(email)
	if err != nil {
		ws = &model.Workspace{
			Handle:    model.NewWorkspaceHandle(),
			Name:      email,
			Email:     email,
			Plan:      "free",
			CreatedAt: time.Now(),
		}
		if err := a.store.CreateWorkspace(ws); err != nil {
			return nil, fmt.Errorf("failed to create workspace: %w", err)
		}
	}

	// Generate token
	token := &model.Token{
		Token:     genToken(a.smtp, email),
		Workspace: ws.Handle,
		Email:     email,
		CreatedAt: time.Now(),
	}
	if err := a.store.SaveToken(token); err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	// Clean up OTP
	_ = a.store.DeleteOTP(email)

	return token, nil
}

// ValidateToken checks if a token is valid and returns the associated token record.
func (a *Auth) ValidateToken(token string) (*model.Token, error) {
	return a.store.GetToken(token)
}

// genOTPCode generates a 6-digit OTP code.
func genOTPCode() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	code := int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
	return fmt.Sprintf("%06d", code%1000000)
}

// genToken generates a hex token.
func genToken(secret, email string) string {
	h := sha256.New()
	h.Write([]byte(secret + email + time.Now().String()))
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}
