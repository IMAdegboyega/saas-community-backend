package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration
type Config struct {
	// Server
	Environment string
	Port        string
	BaseURL     string

	// Database
	DatabaseURL string

	// Redis (optional)
	RedisURL string

	// JWT
	JWTSecret          string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration

	// Security
	BCryptCost int
	Enable2FA  bool

	// OTP
	OTPLength      int
	OTPExpiry      time.Duration
	MaxOTPAttempts int

	// Email
	EmailProvider  string // "sendgrid", "smtp", "mock"
	SendGridAPIKey string
	SMTPHost       string
	SMTPPort       int
	SMTPUsername   string
	SMTPPassword   string
	EmailFrom      string

	// SMS
	SMSProvider       string // "twilio", "mock"
	TwilioAccountSID  string
	TwilioAuthToken   string
	TwilioPhoneNumber string

	// Storage
	UseS3          bool
	S3Bucket       string
	S3Region       string
	LocalUploadDir string

	// Push Notifications
	FCMCredentialsFile string

	// Rate Limiting
	RateLimitRequests int
	RateLimitWindow   time.Duration
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		// Server
		Environment: getEnv("ENVIRONMENT", "development"),
		Port:        getEnv("PORT", "8080"),
		BaseURL:     getEnv("BASE_URL", "http://localhost:8080"),

		// Database
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/kiekky?sslmode=disable"),

		// Redis
		RedisURL: getEnv("REDIS_URL", ""),

		// JWT
		JWTSecret:          getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		AccessTokenExpiry:  getDuration("ACCESS_TOKEN_EXPIRY", 15*time.Minute),
		RefreshTokenExpiry: getDuration("REFRESH_TOKEN_EXPIRY", 7*24*time.Hour),

		// Security
		BCryptCost: getIntEnv("BCRYPT_COST", 12),
		Enable2FA:  getBoolEnv("ENABLE_2FA", false),

		// OTP
		OTPLength:      getIntEnv("OTP_LENGTH", 6),
		OTPExpiry:      getDuration("OTP_EXPIRY", 10*time.Minute),
		MaxOTPAttempts: getIntEnv("MAX_OTP_ATTEMPTS", 3),

		// Email
		EmailProvider:  getEnv("EMAIL_PROVIDER", "mock"),
		SendGridAPIKey: getEnv("SENDGRID_API_KEY", ""),
		SMTPHost:       getEnv("SMTP_HOST", ""),
		SMTPPort:       getIntEnv("SMTP_PORT", 587),
		SMTPUsername:   getEnv("SMTP_USERNAME", ""),
		SMTPPassword:   getEnv("SMTP_PASSWORD", ""),
		EmailFrom:      getEnv("EMAIL_FROM", "noreply@kiekky.com"),

		// SMS
		SMSProvider:       getEnv("SMS_PROVIDER", "mock"),
		TwilioAccountSID:  getEnv("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:   getEnv("TWILIO_AUTH_TOKEN", ""),
		TwilioPhoneNumber: getEnv("TWILIO_PHONE_NUMBER", ""),

		// Storage
		UseS3:          getBoolEnv("USE_S3", false),
		S3Bucket:       getEnv("S3_BUCKET", ""),
		S3Region:       getEnv("S3_REGION", "us-east-1"),
		LocalUploadDir: getEnv("LOCAL_UPLOAD_DIR", "./uploads"),

		// Push Notifications
		FCMCredentialsFile: getEnv("FCM_CREDENTIALS_FILE", ""),

		// Rate Limiting
		RateLimitRequests: getIntEnv("RATE_LIMIT_REQUESTS", 100),
		RateLimitWindow:   getDuration("RATE_LIMIT_WINDOW", time.Minute),
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.JWTSecret == "" || c.JWTSecret == "your-secret-key-change-in-production" {
		if c.Environment == "production" {
			return fmt.Errorf("JWT_SECRET must be set in production")
		}
	}
	return nil
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
