package internal

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/xid"
)

// APIErrorResponse represents the standardized JSON error payload returned to clients.
type APIErrorResponse struct {
	Error   string            `json:"error"`
	Code    string            `json:"code,omitempty"`
	ErrorID string            `json:"error_id,omitempty"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// RespondInternalError logs full technical diagnostics securely on the server
// and returns an opaque, user-safe error message with a unique error_id to the client.
func RespondInternalError(c *gin.Context, app *Application, err error, technicalContext string) {
	errorID := xid.New().String()

	app.ErrorLog.Printf(
		"[ERROR_ID: %s] [%s %s] ClientIP: %s | Context: %s | Details: %v\n",
		errorID,
		c.Request.Method,
		c.Request.URL.Path,
		c.ClientIP(),
		technicalContext,
		err,
	)

	c.JSON(http.StatusInternalServerError, APIErrorResponse{
		Error:   "An internal server error occurred. Please contact support with the error ID.",
		Code:    "INTERNAL_SERVER_ERROR",
		ErrorID: errorID,
	})
}

// RespondClientError returns a clean, structured client error (4xx) without leaking server internals.
func RespondClientError(c *gin.Context, status int, code, message string) {
	c.JSON(status, APIErrorResponse{
		Error: message,
		Code:  code,
	})
}

// RespondValidationError returns formatted field-level validation errors.
func RespondValidationError(c *gin.Context, fields map[string]string) {
	c.JSON(http.StatusBadRequest, APIErrorResponse{
		Error:  "Validation failed",
		Code:   "VALIDATION_ERROR",
		Fields: fields,
	})
}
