// Package config reads the service configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
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
	if len(missing) > 0 {
		return c, fmt.Errorf("missing environment variables: %v", missing)
	}
	return c, nil
}
