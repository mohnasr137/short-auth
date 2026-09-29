package middlewares

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"mohnasr137/short-auth/internal"

	"github.com/gin-gonic/gin"
)

type KeycloakClaims struct {
	Subject           string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
	Picture           string `json:"picture"`
	RealmAccess       struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

func AuthMiddleware(app *internal.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		var rawToken string

		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
				rawToken = parts[1]
			} else {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "Header format must be: Bearer <token>",
					"code":  "INVALID_HEADER_FORMAT",
				})
				return
			}
		}

		// Fallback to HttpOnly cookie if Authorization header is not provided (standard browser flow)
		if rawToken == "" {
			if cookieToken, err := c.Cookie("access_token"); err == nil && cookieToken != "" {
				rawToken = cookieToken
			}
		}

		if rawToken == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization token missing (provide Bearer header or access_token cookie)",
				"code":  "AUTH_TOKEN_MISSING",
			})
			return
		}

		idToken, err := app.Verifier.Verify(c.Request.Context(), rawToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired authentication token",
				"code":  "INVALID_TOKEN",
			})
			return
		}

		// Validate token issuer belongs to our configured realm
		expectedRealmSuffix := "/realms/" + app.Config.KeycloakRealm
		if !strings.HasSuffix(idToken.Issuer, expectedRealmSuffix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token issuer",
				"code":  "INVALID_ISSUER",
			})
			return
		}

		var claims KeycloakClaims
		if err := idToken.Claims(&claims); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Failed to parse token claims",
				"code":  "INVALID_CLAIMS",
			})
			return
		}

		displayName := claims.Name
		if displayName == "" {
			fullName := strings.TrimSpace(claims.GivenName + " " + claims.FamilyName)
			if fullName != "" {
				displayName = fullName
			} else {
				displayName = claims.PreferredUsername
			}
		}

		roles := claims.RealmAccess.Roles
		if roles == nil {
			roles = []string{}
		}

		userID := claims.Subject
		if userID == "" {
			userID = idToken.Subject
		}
		c.Set("userID", userID)
		c.Set("username", claims.PreferredUsername)
		c.Set("email", claims.Email)
		c.Set("name", displayName)
		c.Set("firstName", claims.GivenName)
		c.Set("lastName", claims.FamilyName)
		c.Set("picture", claims.Picture)
		c.Set("roles", roles)

		c.Next()
	}
}

func RequireRole(targetRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		rolesInterface, exists := c.Get("roles")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "No roles found in token",
				"code":  "NO_ROLES",
			})
			return
		}

		roles, ok := rolesInterface.([]string)
		if !ok || !slices.Contains(roles, targetRole) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("Access denied: required role '%s' not present", targetRole),
				"code":  "FORBIDDEN",
			})
			return
		}

		c.Next()
	}
}
