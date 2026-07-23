package config

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
)

// Config holds all service configuration.
type Config struct {
	Addr   string // listen address
	Data   string // data directory for JSON files
	SMTP   string // SMTP host:port for OTP email (empty = log to stderr)
	Secret string // token signing secret
}

// Load reads configuration from defaults, env, and flags.
func Load() *Config {
	c := &Config{}

	// Defaults
	c.Addr = ":7705"
	c.Data = "./formkit-data"
	c.SMTP = ""
	c.Secret = ""

	// Flags
	flag.StringVar(&c.Addr, "addr", c.Addr, "listen address")
	flag.StringVar(&c.Data, "data", c.Data, "data directory for JSON files")
	flag.StringVar(&c.SMTP, "smtp", c.SMTP, "SMTP host:port for OTP email (empty = log to stderr)")
	flag.StringVar(&c.Secret, "secret", c.Secret, "token signing secret (auto-generated if empty)")
	flag.Parse()

	// Env overrides
	if v := os.Getenv("FORMKIT_ADDR"); v != "" {
		c.Addr = v
	}
	if v := os.Getenv("FORMKIT_DATA"); v != "" {
		c.Data = v
	}
	if v := os.Getenv("FORMKIT_SMTP"); v != "" {
		c.SMTP = v
	}
	if v := os.Getenv("FORMKIT_SECRET"); v != "" {
		c.Secret = v
	}

	return c
}

// Sanitize fills in missing config with sensible defaults.
func (c *Config) Sanitize() {
	if c.Secret == "" {
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		c.Secret = hex.EncodeToString(b)
		fmt.Fprintf(os.Stderr, "FORMKIT_SECRET not set — generated random secret for this session.\n")
	}
}
