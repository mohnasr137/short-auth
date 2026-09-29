package internal

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
)

var (
	hasLetter = regexp.MustCompile(`[a-zA-Z]`)
	hasDigit  = regexp.MustCompile(`[0-9]`)
)

// ValidatePasswordStrength checks for production-grade password strength:
// Minimum 8 characters, at least one letter, at least one number, and no leading/trailing whitespace.
func ValidatePasswordStrength(password string) error {
	if strings.TrimSpace(password) != password {
		return errors.New("password cannot start or end with whitespace")
	}
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}
	if !hasLetter.MatchString(password) {
		return errors.New("password must contain at least one letter")
	}
	if !hasDigit.MatchString(password) {
		return errors.New("password must contain at least one number")
	}
	return nil
}

// FormatValidationErrors converts raw validator.ValidationErrors into client-friendly messages.
func FormatValidationErrors(err error) map[string]string {
	fieldErrors := make(map[string]string)

	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		for _, fe := range ve {
			jsonField := toSnakeCase(fe.Field())
			fieldErrors[jsonField] = msgForTag(jsonField, fe.Tag(), fe.Param())
		}
		return fieldErrors
	}

	// Fallback for general unmarshalling / syntax errors
	fieldErrors["request"] = "Invalid request payload format: " + err.Error()
	return fieldErrors
}

func msgForTag(field, tag, param string) string {
	friendlyName := formatFieldName(field)

	switch tag {
	case "required":
		return fmt.Sprintf("%s is required", friendlyName)
	case "email":
		return "Please provide a valid email address"
	case "min":
		return fmt.Sprintf("%s must be at least %s characters long", friendlyName, param)
	case "max":
		return fmt.Sprintf("%s must not exceed %s characters", friendlyName, param)
	default:
		return fmt.Sprintf("%s is invalid", friendlyName)
	}
}

func formatFieldName(field string) string {
	parts := strings.Split(field, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// toSnakeCase converts PascalCase (e.g. FirstName) to snake_case (e.g. first_name)
func toSnakeCase(str string) string {
	var result []rune
	for i, r := range str {
		if unicode.IsUpper(r) {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, unicode.ToLower(r))
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}
