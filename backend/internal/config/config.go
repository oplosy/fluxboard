// Package config parses 12-factor env config once at startup \(fail-fast, secrets redacted in String\(\)\)\. process configuration once at startup (12-factor:
// env only). Required keys missing => fail fast. Secrets never reach logs:
// String() redacts them (docs/10-INFRA-DEVOPS.md §2).
package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

// Config is the fully-resolved runtime configuration. Construct it via Load;
// treat it as immutable afterwards. Fields tagged secret are masked by String().
type Config struct {
	AppEnv            string `envconfig:"APP_ENV" default:"dev"`
	HTTPAddr          string `envconfig:"HTTP_ADDR" default:":8080"`
	WorkerMetricsAddr string `envconfig:"WORKER_METRICS_ADDR" default:":8081"`

	// Database. app role for runtime queries; owner role only for migrations.
	DatabaseURL        string `envconfig:"DATABASE_URL" required:"true"`
	DatabaseURLMigrate string `envconfig:"DATABASE_URL_MIGRATE"`

	RedisAddr string `envconfig:"REDIS_ADDR" required:"true"`

	// Auth (Phase 1). PEM of the ES256 private key that signs access tokens.
	JWTPrivateKeyPEM string `envconfig:"JWT_PRIVATE_KEY_PEM" secret:"true"`
	TOTPEncKey       string `envconfig:"TOTP_ENC_KEY" secret:"true"`

	// Google OAuth2 (PKCE). Empty client id => the Google login routes 403.
	GoogleClientID     string `envconfig:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `envconfig:"GOOGLE_CLIENT_SECRET" secret:"true"`
	GoogleRedirectURL  string `envconfig:"GOOGLE_REDIRECT_URL" default:"http://localhost:8080/api/v1/auth/oauth/google/callback"`

	// GitHub OAuth App (PKCE). Empty client id => GitHub sign-in is hidden/off.
	GitHubClientID     string `envconfig:"GITHUB_CLIENT_ID"`
	GitHubClientSecret string `envconfig:"GITHUB_CLIENT_SECRET" secret:"true"`
	GitHubRedirectURL  string `envconfig:"GITHUB_REDIRECT_URL" default:"http://localhost:8080/api/v1/auth/oauth/github/callback"`

	// TrustedProxyHops is the number of reverse proxies in front of the API that
	// append to X-Forwarded-For (1 behind a single Caddy/nginx/ALB). 0 ignores
	// the header, so a client cannot forge its IP to dodge per-IP throttles.
	TrustedProxyHops int `envconfig:"TRUSTED_PROXY_HOPS" default:"0"`

	// Billing (Phase 4). StripeMode selects the gateway: "stub" (dev fake, no
	// external calls; webhook signatures are a shared-secret HMAC of the body)
	// or "live" (real stripe-go against the test/prod API). In stub mode
	// StripeWebhookSecret doubles as the HMAC key.
	StripeMode          string `envconfig:"STRIPE_MODE" default:"stub"`
	StripeSecretKey     string `envconfig:"STRIPE_SECRET_KEY" secret:"true"`
	StripeWebhookSecret string `envconfig:"STRIPE_WEBHOOK_SECRET" secret:"true"`
	StripePriceSeed     bool   `envconfig:"STRIPE_PRICE_SEED" default:"false"`

	// Object storage.
	MinIOEndpoint  string `envconfig:"MINIO_ENDPOINT"`
	MinIOAccessKey string `envconfig:"MINIO_ACCESS_KEY" secret:"true"`
	MinIOSecretKey string `envconfig:"MINIO_SECRET_KEY" secret:"true"`
	MinIOBucket    string `envconfig:"MINIO_BUCKET" default:"fluxboard"`
	MinIOUseSSL    bool   `envconfig:"MINIO_USE_SSL" default:"false"`

	// Mail (SMTP -> Mailpit in local dev).
	SMTPHost     string `envconfig:"SMTP_HOST"`
	SMTPPort     int    `envconfig:"SMTP_PORT" default:"1025"`
	SMTPUser     string `envconfig:"SMTP_USER"`
	SMTPPassword string `envconfig:"SMTP_PASSWORD" secret:"true"`
	SMTPFrom     string `envconfig:"SMTP_FROM" default:"no-reply@fluxboard.local"`

	WebOrigin string `envconfig:"WEB_ORIGIN" default:"http://localhost:3000"`
	// WebExtraOrigins are further browser origins CORS accepts (comma-separated),
	// e.g. the local production build on :3002. Links and emails use WebOrigin.
	WebExtraOrigins []string `envconfig:"WEB_EXTRA_ORIGINS"`

	// Governed AI layer (ADR-025, FR-AI-001..008). Provider selects the
	// ai.Provider implementation: "mock" (default — deterministic, keyless,
	// boot-safe). Live providers ("claude", "azure") arrive later; unknown
	// values fall back to mock at wiring (never fail boot). MonthlyCap bounds
	// AI actions per org per calendar month (402 ai_actions at breach).
	AIProvider   string `envconfig:"AI_PROVIDER" default:"mock"`
	AIMonthlyCap int    `envconfig:"AI_MONTHLY_CAP" default:"200"`
}

// Recognised APP_ENV values. Anything else is rejected so a typo such as
// "production" cannot silently boot with dev fallbacks.
const (
	EnvDev  = "dev"
	EnvProd = "prod"
)

// Load reads configuration from the environment and returns it, or an error
// naming the first missing/invalid key.
func Load() (*Config, error) {
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if c.AppEnv != EnvDev && c.AppEnv != EnvProd {
		return nil, fmt.Errorf("config: APP_ENV=%q, want %q or %q", c.AppEnv, EnvDev, EnvProd)
	}
	return &c, nil
}

// IsProd reports whether the process runs in a production-shaped environment.
func (c *Config) IsProd() bool { return c.AppEnv == EnvProd }

// ValidateAPI rejects a prod config the API would otherwise boot with a dev
// fallback: an ephemeral JWT key (sessions break on restart and across
// replicas), 2FA silently off, mail bodies with one-time links logged instead
// of sent, attachments off, or localhost links and redirects. No-op in dev.
func (c *Config) ValidateAPI() error {
	if !c.IsProd() {
		return nil
	}
	errs := append(c.prodCommon(),
		required("JWT_PRIVATE_KEY_PEM", c.JWTPrivateKeyPEM),
		required("TOTP_ENC_KEY", c.TOTPEncKey),
	)
	if c.GoogleClientID != "" && !strings.HasPrefix(c.GoogleRedirectURL, "https://") {
		errs = append(errs, fmt.Errorf("GOOGLE_REDIRECT_URL must be an https URL, got %q", c.GoogleRedirectURL))
	}
	if c.GitHubClientID != "" && !strings.HasPrefix(c.GitHubRedirectURL, "https://") {
		errs = append(errs, fmt.Errorf("GITHUB_REDIRECT_URL must be an https URL, got %q", c.GitHubRedirectURL))
	}
	if c.TrustedProxyHops < 0 {
		errs = append(errs, fmt.Errorf("TRUSTED_PROXY_HOPS must be >= 0, got %d", c.TrustedProxyHops))
	}
	return prodErr(errs)
}

// ValidateWorker is ValidateAPI for the worker, which sends mail, GCs objects
// and renders links but never signs tokens. No-op in dev.
func (c *Config) ValidateWorker() error {
	if !c.IsProd() {
		return nil
	}
	return prodErr(c.prodCommon())
}

// prodCommon checks the keys both processes need in prod.
func (c *Config) prodCommon() []error {
	errs := []error{
		required("SMTP_HOST", c.SMTPHost),
		required("MINIO_ENDPOINT", c.MinIOEndpoint),
		required("MINIO_ACCESS_KEY", c.MinIOAccessKey),
		required("MINIO_SECRET_KEY", c.MinIOSecretKey),
	}
	if !strings.HasPrefix(c.WebOrigin, "https://") {
		errs = append(errs, fmt.Errorf("WEB_ORIGIN must be an https URL, got %q", c.WebOrigin))
	}
	for _, o := range c.WebExtraOrigins {
		if !strings.HasPrefix(o, "https://") {
			errs = append(errs, fmt.Errorf("WEB_EXTRA_ORIGINS must hold https URLs, got %q", o))
		}
	}
	// Mail goes out from the operator's real domain (report B09), never the dev default.
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(c.SMTPFrom)), ".local") || strings.TrimSpace(c.SMTPFrom) == "" {
		errs = append(errs, fmt.Errorf("SMTP_FROM must be an address on the operator's domain, got %q", c.SMTPFrom))
	}
	// The stub gateway fakes checkout and accepts HMAC-signed fake webhooks.
	if c.StripeMode != "live" {
		errs = append(errs, fmt.Errorf("STRIPE_MODE must be live, got %q", c.StripeMode))
	}
	errs = append(errs,
		required("STRIPE_SECRET_KEY", c.StripeSecretKey),
		required("STRIPE_WEBHOOK_SECRET", c.StripeWebhookSecret),
	)
	return errs
}

func required(key, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", key)
	}
	return nil
}

// prodErr joins every failed check so one boot reports all missing keys.
func prodErr(errs []error) error {
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("config: APP_ENV=prod: %w", err)
	}
	return nil
}

// String renders the config with every secret masked, safe to log at startup.
func (c *Config) String() string {
	var b strings.Builder
	b.WriteString("Config{")
	fmt.Fprintf(&b, "AppEnv=%s HTTPAddr=%s WorkerMetricsAddr=%s ", c.AppEnv, c.HTTPAddr, c.WorkerMetricsAddr)
	fmt.Fprintf(&b, "DatabaseURL=%s DatabaseURLMigrate=%s RedisAddr=%s ",
		redactDSN(c.DatabaseURL), redactDSN(c.DatabaseURLMigrate), c.RedisAddr)
	fmt.Fprintf(&b, "JWTPrivateKeyPEM=%s TOTPEncKey=%s ", mask(c.JWTPrivateKeyPEM), mask(c.TOTPEncKey))
	fmt.Fprintf(&b, "GoogleClientID=%s GoogleClientSecret=%s GoogleRedirectURL=%s ",
		c.GoogleClientID, mask(c.GoogleClientSecret), c.GoogleRedirectURL)
	fmt.Fprintf(&b, "GitHubClientID=%s GitHubClientSecret=%s GitHubRedirectURL=%s TrustedProxyHops=%d ",
		c.GitHubClientID, mask(c.GitHubClientSecret), c.GitHubRedirectURL, c.TrustedProxyHops)
	fmt.Fprintf(&b, "StripeMode=%s StripeSecretKey=%s StripeWebhookSecret=%s StripePriceSeed=%t ",
		c.StripeMode, mask(c.StripeSecretKey), mask(c.StripeWebhookSecret), c.StripePriceSeed)
	fmt.Fprintf(&b, "MinIOEndpoint=%s MinIOAccessKey=%s MinIOSecretKey=%s MinIOBucket=%s MinIOUseSSL=%t ",
		c.MinIOEndpoint, mask(c.MinIOAccessKey), mask(c.MinIOSecretKey), c.MinIOBucket, c.MinIOUseSSL)
	fmt.Fprintf(&b, "SMTPHost=%s SMTPPort=%d SMTPUser=%s SMTPPassword=%s SMTPFrom=%s ",
		c.SMTPHost, c.SMTPPort, c.SMTPUser, mask(c.SMTPPassword), c.SMTPFrom)
	fmt.Fprintf(&b, "WebOrigin=%s AIProvider=%s AIMonthlyCap=%d}", c.WebOrigin, c.AIProvider, c.AIMonthlyCap)
	return b.String()
}

// mask returns a fixed redaction marker for non-empty secrets, "" for unset.
func mask(s string) string {
	if s == "" {
		return "\"\""
	}
	return "\"***\""
}

// redactDSN keeps the DSN shape but hides any embedded password.
func redactDSN(dsn string) string {
	if dsn == "" {
		return "\"\""
	}
	// postgres://user:pass@host/db -> postgres://user:***@host/db
	at := strings.LastIndex(dsn, "@")
	scheme := strings.Index(dsn, "://")
	if at == -1 || scheme == -1 {
		return "\"***\""
	}
	cred := dsn[scheme+3 : at]
	if colon := strings.Index(cred, ":"); colon != -1 {
		return dsn[:scheme+3] + cred[:colon] + ":***" + dsn[at:]
	}
	return dsn
}
