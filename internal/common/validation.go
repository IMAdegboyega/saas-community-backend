package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

var (
	validate *validator.Validate

	// Common regex patterns
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,30}$`)
	phoneRegex    = regexp.MustCompile(`^\+?[1-9]\d{1,14}$`)
)

func init() {
	validate = validator.New()

	// Register custom validations
	validate.RegisterValidation("username", validateUsername)
	validate.RegisterValidation("phone", validatePhone)
}

// ValidateStruct validates a struct using validator tags
func ValidateStruct(s interface{}) map[string]string {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	errors := make(map[string]string)
	for _, err := range err.(validator.ValidationErrors) {
		field := strings.ToLower(err.Field())
		errors[field] = getValidationMessage(err)
	}
	return errors
}

// DecodeAndValidate decodes JSON request body and validates it
func DecodeAndValidate(r *http.Request, dst interface{}) map[string]string {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return map[string]string{"body": "Invalid JSON format"}
	}
	return ValidateStruct(dst)
}

// Custom validation functions
func validateUsername(fl validator.FieldLevel) bool {
	return usernameRegex.MatchString(fl.Field().String())
}

func validatePhone(fl validator.FieldLevel) bool {
	phone := fl.Field().String()
	if phone == "" {
		return true // Allow empty (use `required` tag if needed)
	}
	return phoneRegex.MatchString(phone)
}

// Get human-readable validation messages
func getValidationMessage(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", err.Field())
	case "email":
		return "Invalid email format"
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", err.Field(), err.Param())
	case "max":
		return fmt.Sprintf("%s must not exceed %s characters", err.Field(), err.Param())
	case "username":
		return "Username must be 3-30 characters, alphanumeric and underscores only"
	case "phone":
		return "Invalid phone number format"
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", err.Field(), err.Param())
	case "url":
		return "Invalid URL format"
	case "gte":
		return fmt.Sprintf("%s must be at least %s", err.Field(), err.Param())
	case "lte":
		return fmt.Sprintf("%s must be at most %s", err.Field(), err.Param())
	default:
		return fmt.Sprintf("%s is invalid", err.Field())
	}
}

// ValidateEmail validates email format
func ValidateEmail(email string) bool {
	return validate.Var(email, "required,email") == nil
}

// ValidateUsername validates username format
func ValidateUsername(username string) bool {
	return usernameRegex.MatchString(username)
}

// ValidatePhone validates phone number format
func ValidatePhone(phone string) bool {
	return phoneRegex.MatchString(phone)
}

// SanitizeString trims whitespace and normalizes a string
func SanitizeString(s string) string {
	return strings.TrimSpace(s)
}

// SanitizeEmail normalizes an email address
func SanitizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// SanitizeUsername normalizes a username
func SanitizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}
