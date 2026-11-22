// Package config reads the service configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Listen      string
	DatabaseURL string
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3UseTLS    bool
	ValkeyAddr  string

	// AdminPassword creates the account "admin" when the database has no account.
	AdminPassword string
	// StorageLimit in bytes for garbage collection (ADR-0005). 0 means no limit.
	StorageLimit int64
}

// Load returns the configuration, or an error that names every missing variable.
func Load() (Config, error) {
	var missing []string
	need := func(name string) string {
		v := os.Getenv(name)
		if v == "" {
			missing = append(missing, name)
		}
		return v
	}
	c := Config{
		Listen:      os.Getenv("GINNUNGAGAP_LISTEN"),
		DatabaseURL: need("DATABASE_URL"),
		S3Endpoint:  need("S3_ENDPOINT"),
		S3AccessKey: need("S3_ACCESS_KEY"),
		S3SecretKey: need("S3_SECRET_KEY"),
		S3Bucket:    need("S3_BUCKET"),
		ValkeyAddr:  need("VALKEY_ADDR"),
	}
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if v := os.Getenv("S3_USE_TLS"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return c, fmt.Errorf("S3_USE_TLS: %w", err)
		}
		c.S3UseTLS = b
	}
	c.AdminPassword = os.Getenv("GINNUNGAGAP_ADMIN_PASSWORD")
	if v := os.Getenv("GC_STORAGE_LIMIT"); v != "" {
		n, err := ParseSize(v)
		if err != nil {
			return c, fmt.Errorf("GC_STORAGE_LIMIT: %w", err)
		}
		c.StorageLimit = n
	}
	if len(missing) > 0 {
		return c, fmt.Errorf("missing environment variables: %v", missing)
	}
	return c, nil
}

var units = []struct {
	suffix string
	n      int64
}{
	{"TiB", 1 << 40}, {"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10},
	{"TB", 1e12}, {"GB", 1e9}, {"MB", 1e6}, {"KB", 1e3}, {"B", 1},
}

// ParseSize reads a size such as "200GiB", "50GB", or "1048576".
func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	mult := int64(1)
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			s, mult = strings.TrimSpace(strings.TrimSuffix(s, u.suffix)), u.n
			break
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	return n * mult, nil
}
