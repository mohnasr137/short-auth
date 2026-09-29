package middlewares

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

// CORS returns a middleware that handles Cross-Origin Resource Sharing (CORS) securely.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		if origin != "" {
			// Check if origin is allowed or if wildcard is enabled
			if slices.Contains(allowedOrigins, "*") || slices.Contains(allowedOrigins, origin) {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD")
				c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Accept, Cache-Control, X-Requested-With")
				c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type, Retry-After")
				c.Header("Access-Control-Max-Age", "86400") // 24 hours preflight cache
			}
		}

		// Respond immediately to Preflight OPTIONS requests
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
