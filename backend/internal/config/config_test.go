package config

import (
	"strings"
	"testing"
)

// prodConfig is a prod config that passes both validators.
func prodConfig() *Config {
	return &Config{
		AppEnv:           EnvProd,
		JWTPrivateKeyPEM: "-----BEGIN EC PRIVATE KEY-----",
		TOTPEncKey:       "key",
		SMTPHost:         "smtp.example.com",
		MinIOEndpoint:    "s3.example.com",
		MinIOAccessKey:   "access",
		MinIOSecretKey:   "secret",
		WebOrigin:        "https://app.example.com",
	}
}

func TestValidateProdComplete(t *testing.T) {
	c := prodConfig()
	if err := c.ValidateAPI(); err != nil {
		t.Fatalf("ValidateAPI: %v", err)
	}
	if err := c.ValidateWorker(); err != nil {
		t.Fatalf("ValidateWorker: %v", err)
	}
}

func TestValidateAPIProdRejectsDevFallbacks(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"no jwt key", func(c *Config) { c.JWTPrivateKeyPEM = "" }, "JWT_PRIVATE_KEY_PEM is required"},
		{"no totp key", func(c *Config) { c.TOTPEncKey = "" }, "TOTP_ENC_KEY is required"},
		{"no smtp", func(c *Config) { c.SMTPHost = "" }, "SMTP_HOST is required"},
		{"no minio", func(c *Config) { c.MinIOEndpoint = "" }, "MINIO_ENDPOINT is required"},
		{"no minio secret", func(c *Config) { c.MinIOSecretKey = "" }, "MINIO_SECRET_KEY is required"},
		{"http origin", func(c *Config) { c.WebOrigin = "http://localhost:3000" }, "WEB_ORIGIN must be an https URL"},
		{"google http redirect", func(c *Config) {
			c.GoogleClientID = "id"
			c.GoogleRedirectURL = "http://localhost:8080/cb"
		}, "GOOGLE_REDIRECT_URL must be an https URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := prodConfig()
			tt.mutate(c)
			err := c.ValidateAPI()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateAPI err = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestValidateAPIProdReportsEveryMissingKey(t *testing.T) {
	err := (&Config{AppEnv: EnvProd}).ValidateAPI()
	if err == nil {
		t.Fatal("empty prod config passed validation")
	}
	for _, key := range []string{"JWT_PRIVATE_KEY_PEM", "TOTP_ENC_KEY", "SMTP_HOST", "MINIO_ENDPOINT", "WEB_ORIGIN"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error does not mention %s: %v", key, err)
		}
	}
}

func TestValidateWorkerProdDoesNotNeedAuthKeys(t *testing.T) {
	c := prodConfig()
	c.JWTPrivateKeyPEM, c.TOTPEncKey = "", ""
	if err := c.ValidateWorker(); err != nil {
		t.Fatalf("worker should not need auth keys: %v", err)
	}
	c.SMTPHost = ""
	if err := c.ValidateWorker(); err == nil || !strings.Contains(err.Error(), "SMTP_HOST") {
		t.Fatalf("ValidateWorker err = %v, want SMTP_HOST required", err)
	}
}

func TestValidateDevKeepsFallbacks(t *testing.T) {
	c := &Config{AppEnv: EnvDev, WebOrigin: "http://localhost:3000"}
	if err := c.ValidateAPI(); err != nil {
		t.Fatalf("dev ValidateAPI: %v", err)
	}
	if err := c.ValidateWorker(); err != nil {
		t.Fatalf("dev ValidateWorker: %v", err)
	}
}

func TestLoadRejectsUnknownAppEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost/db")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("APP_ENV", "production")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), `APP_ENV="production"`) {
		t.Fatalf("Load err = %v, want unknown APP_ENV rejected", err)
	}
	t.Setenv("APP_ENV", EnvDev)
	if _, err := Load(); err != nil {
		t.Fatalf("Load dev: %v", err)
	}
}
