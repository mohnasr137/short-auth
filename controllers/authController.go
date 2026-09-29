package controllers

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"mohnasr137/short-auth/internal"
	"mohnasr137/short-auth/middlewares"
	"mohnasr137/short-auth/services"

	"github.com/gin-gonic/gin"
)

func RegisterHandler(app *internal.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input services.RegisterInput
		if err := c.ShouldBindJSON(&input); err != nil {
			internal.RespondValidationError(c, internal.FormatValidationErrors(err))
			return
		}

		if err := internal.ValidatePasswordStrength(input.Password); err != nil {
			internal.RespondValidationError(c, map[string]string{
				"password": err.Error(),
			})
			return
		}

		serviceToken, err := services.GetServiceToken(app, c.Request.Context())
		if err != nil {
			internal.RespondInternalError(c, app, err, "Failed to obtain Keycloak service token")
			return
		}

		payload := map[string]interface{}{
			"username":      input.Username,
			"email":         input.Email,
			"firstName":     input.FirstName,
			"lastName":      input.LastName,
			"enabled":       true,
			"emailVerified": true,
			"credentials": []map[string]interface{}{
				{
					"type":      "password",
					"value":     input.Password,
					"temporary": false,
				},
			},
		}

		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			internal.RespondInternalError(c, app, err, "Failed to serialize registration payload")
			return
		}

		adminURL := fmt.Sprintf("%s/admin/realms/%s/users", app.Config.KeycloakBaseURL, app.Config.KeycloakRealm)
		req, err := http.NewRequestWithContext(c.Request.Context(), "POST", adminURL, bytes.NewBuffer(bodyBytes))
		if err != nil {
			internal.RespondInternalError(c, app, err, "Failed to create registration request")
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+serviceToken)

		resp, err := app.HTTPClient.Do(req)
		if err != nil {
			internal.RespondInternalError(c, app, err, "Failed to reach Keycloak identity provider")
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusConflict {
			internal.RespondClientError(c, http.StatusConflict, "USER_ALREADY_EXISTS", "A user with this username or email already exists")
			return
		}

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			internal.RespondInternalError(c, app, fmt.Errorf("Keycloak status %d: %s", resp.StatusCode, string(body)), "Keycloak user creation rejected")
			return
		}

		// Extract new Keycloak user ID from Location header (e.g., .../users/{id})
		var userID string
		locationHeader := resp.Header.Get("Location")
		if locationHeader != "" {
			parts := strings.Split(locationHeader, "/")
			userID = parts[len(parts)-1]
		}

		// If Location header was missing or empty, search by email to retrieve the ID
		if userID == "" {
			if u, err := services.FindUserByEmail(app, c.Request.Context(), serviceToken, input.Email); err == nil && u != nil {
				userID = u.ID
			}
		}

		app.InfoLog.Printf("User %s registered successfully (ID: %s)\n", input.Username, userID)
		c.JSON(http.StatusCreated, gin.H{
			"message": "User registered successfully",
			"user_id": userID,
		})
	}
}

func ChangePasswordHandler(app *internal.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		username := c.GetString("username")

		var input services.ChangePasswordInput
		if err := c.ShouldBindJSON(&input); err != nil {
			internal.RespondValidationError(c, internal.FormatValidationErrors(err))
			return
		}

		if err := internal.ValidatePasswordStrength(input.NewPassword); err != nil {
			internal.RespondValidationError(c, map[string]string{
				"new_password": err.Error(),
			})
			return
		}

		if err := services.VerifyOldPassword(app, c.Request.Context(), username, input.OldPassword); err != nil {
			internal.RespondClientError(c, http.StatusBadRequest, "INVALID_CREDENTIALS", "Old password is incorrect")
			return
		}

		serviceToken, err := services.GetServiceToken(app, c.Request.Context())
		if err != nil {
			internal.RespondInternalError(c, app, err, "Failed to obtain Keycloak service token")
			return
		}

		payload := map[string]interface{}{
			"type":      "password",
			"value":     input.NewPassword,
			"temporary": false,
		}

		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			internal.RespondInternalError(c, app, err, "Failed to serialize password reset payload")
			return
		}

		resetURL := fmt.Sprintf("%s/admin/realms/%s/users/%s/reset-password", app.Config.KeycloakBaseURL, app.Config.KeycloakRealm, userID)
		req, err := http.NewRequestWithContext(c.Request.Context(), "PUT", resetURL, bytes.NewBuffer(bodyBytes))
		if err != nil {
			internal.RespondInternalError(c, app, err, "Failed to create password reset request")
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+serviceToken)

		resp, err := app.HTTPClient.Do(req)
		if err != nil {
			internal.RespondInternalError(c, app, err, "Failed to reach Keycloak reset-password endpoint")
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			internal.RespondInternalError(c, app, fmt.Errorf("Keycloak status %d: %s", resp.StatusCode, string(body)), "Keycloak password reset failed")
			return
		}

		app.InfoLog.Printf("Password changed successfully for user: %s\n", username)
		c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
	}
}

// setAuthCookies sets secure, HttpOnly cookies for browser clients (RFC 6265, OWASP guidelines)
func setAuthCookies(c *gin.Context, app *internal.Application, accessToken, refreshToken string, expiresIn int) {
	isProd := strings.ToLower(app.Config.Environment) == "production"

	// Access token cookie (valid for expiresIn seconds)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		"access_token",
		accessToken,
		expiresIn,
		"/",
		"",
		isProd,
		true, // HttpOnly: prevents XSS theft
	)

	// Refresh token cookie (valid for 30 days, scoped to /api/auth)
	if refreshToken != "" {
		c.SetCookie(
			"refresh_token",
			refreshToken,
			30*24*3600,
			"/api/auth",
			"",
			isProd,
			true, // HttpOnly: protects refresh token
		)
	}
}

// clearAuthCookies removes authentication cookies on logout
func clearAuthCookies(c *gin.Context, app *internal.Application) {
	isProd := strings.ToLower(app.Config.Environment) == "production"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("access_token", "", -1, "/", "", isProd, true)
	c.SetCookie("refresh_token", "", -1, "/api/auth", "", isProd, true)
}

// validateRedirectURL checks that the target URL belongs to an allowed origin (prevents Open Redirect attacks)
func validateRedirectURL(app *internal.Application, targetURL string) bool {
	if targetURL == "" {
		return false
	}
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return false
	}
	// Relative path like "/dashboard" is safe
	if parsed.Scheme == "" && parsed.Host == "" {
		return strings.HasPrefix(targetURL, "/")
	}
	targetOrigin := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
	return slices.Contains(app.Config.AllowedOrigins, targetOrigin) || slices.Contains(app.Config.AllowedOrigins, "*")
}

// generateOAuthState generates a cryptographically secure random state token and sets an anti-CSRF cookie
func generateOAuthState(c *gin.Context, app *internal.Application, frontendURL string) (string, error) {
	isProd := strings.ToLower(app.Config.Environment) == "production"

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", err
	}
	rawState := hex.EncodeToString(stateBytes)

	cookieVal := rawState
	if frontendURL != "" {
		cookieVal = fmt.Sprintf("%s|%s", rawState, frontendURL)
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		"oauth_state",
		cookieVal,
		300, // 5 minutes TTL
		"/api/auth/callback",
		"",
		isProd,
		true,
	)

	return rawState, nil
}

func OAuthCallbackHandler(app *internal.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		var code string
		var redirectURI string
		var targetFrontendURL string

		if c.Request.Method == http.MethodGet {
			// Check if OAuth provider returned an error parameter
			if errParam := c.Query("error"); errParam != "" {
				desc := c.DefaultQuery("error_description", "OAuth authorization was rejected or failed")
				internal.RespondClientError(c, http.StatusBadRequest, "OAUTH_ERROR", fmt.Sprintf("%s: %s", errParam, desc))
				return
			}

			// Validate Anti-CSRF OAuth State
			cookieState, cookieErr := c.Cookie("oauth_state")
			isProd := strings.ToLower(app.Config.Environment) == "production"
			c.SetSameSite(http.SameSiteLaxMode)
			c.SetCookie("oauth_state", "", -1, "/api/auth/callback", "", isProd, true)

			queryState := c.Query("state")
			if cookieErr == nil && cookieState != "" {
				parts := strings.SplitN(cookieState, "|", 2)
				expectedState := parts[0]
				if len(parts) > 1 {
					targetFrontendURL = parts[1]
				}

				if queryState == "" || subtle.ConstantTimeCompare([]byte(queryState), []byte(expectedState)) != 1 {
					internal.RespondClientError(c, http.StatusForbidden, "OAUTH_CSRF_DETECTED", "OAuth state mismatch or expired")
					return
				}
			}

			code = c.Query("code")
			if code == "" {
				internal.RespondClientError(c, http.StatusBadRequest, "MISSING_AUTH_CODE", "Missing authorization code in query parameters")
				return
			}

			redirectURI = c.Query("redirect_uri")
			if redirectURI == "" {
				scheme := "http"
				if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
					scheme = "https"
				}
				redirectURI = fmt.Sprintf("%s://%s/api/auth/callback", scheme, c.Request.Host)
			}
		} else {
			// POST method: Expect JSON body for SPA or mobile clients
			var input services.CallbackInput
			if err := c.ShouldBindJSON(&input); err != nil {
				internal.RespondValidationError(c, internal.FormatValidationErrors(err))
				return
			}
			code = input.Code
			redirectURI = input.RedirectURI
		}

		tokenData, err := services.ExchangeCodeForToken(app, c.Request.Context(), code, redirectURI)
		if err != nil {
			app.ErrorLog.Println("Token exchange error:", err)
			internal.RespondClientError(c, http.StatusUnauthorized, "INVALID_AUTH_CODE", "Invalid or expired authorization code")
			return
		}

		// Extract authenticated user profile from token claims
		var userInfo map[string]interface{}
		if idToken, err := app.Verifier.Verify(c.Request.Context(), tokenData.AccessToken); err == nil {
			var claims middlewares.KeycloakClaims
			if err := idToken.Claims(&claims); err == nil && claims.Subject != "" {
				userInfo = map[string]interface{}{
					"id":         claims.Subject,
					"email":      claims.Email,
					"username":   claims.PreferredUsername,
					"first_name": claims.GivenName,
					"last_name":  claims.FamilyName,
				}
			}
		}

		// Set production-grade HttpOnly cookies (OWASP compliant)
		setAuthCookies(c, app, tokenData.AccessToken, tokenData.RefreshToken, tokenData.ExpiresIn)

		// If a target frontend URL was provided, redirect cleanly WITHOUT exposing tokens in URL!
		if targetFrontendURL != "" {
			c.Redirect(http.StatusTemporaryRedirect, targetFrontendURL)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":       "Google authentication successful",
			"access_token":  tokenData.AccessToken,
			"refresh_token": tokenData.RefreshToken,
			"expires_in":    tokenData.ExpiresIn,
			"user":          userInfo,
		})
	}
}

func MeHandler(app *internal.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		username := c.GetString("username")
		email := c.GetString("email")
		name := c.GetString("name")
		firstName := c.GetString("firstName")
		lastName := c.GetString("lastName")
		picture := c.GetString("picture")

		roles, exists := c.Get("roles")
		if !exists || roles == nil {
			roles = []string{}
		}

		c.JSON(http.StatusOK, gin.H{
			"id":         userID,
			"username":   username,
			"email":      email,
			"name":       name,
			"first_name": firstName,
			"last_name":  lastName,
			"picture":    picture,
			"roles":      roles,
		})
	}
}

// VerifyTokenHandler validates a token and returns the caller's identity and roles for downstream microservices.
func VerifyTokenHandler(app *internal.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")
		username := c.GetString("username")
		email := c.GetString("email")
		name := c.GetString("name")
		firstName := c.GetString("firstName")
		lastName := c.GetString("lastName")

		roles, exists := c.Get("roles")
		if !exists || roles == nil {
			roles = []string{}
		}

		c.JSON(http.StatusOK, gin.H{
			"valid":      true,
			"user_id":    userID,
			"username":   username,
			"email":      email,
			"name":       name,
			"first_name": firstName,
			"last_name":  lastName,
			"roles":      roles,
		})
	}
}

func ForgotPasswordHandler(app *internal.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input services.ForgotPasswordInput
		if err := c.ShouldBindJSON(&input); err != nil {
			internal.RespondValidationError(c, internal.FormatValidationErrors(err))
			return
		}

		serviceToken, err := services.GetServiceToken(app, c.Request.Context())
		if err != nil {
			internal.RespondInternalError(c, app, err, "Failed to obtain Keycloak service token")
			return
		}

		user, err := services.FindUserByEmail(app, c.Request.Context(), serviceToken, input.Email)
		if err != nil {
			internal.RespondInternalError(c, app, err, "Failed to query Keycloak for user by email")
			return
		}

		if user == nil {
			internal.RespondClientError(c, http.StatusNotFound, "USER_NOT_FOUND", "No user found with the provided email address")
			return
		}

		if err := services.SendPasswordResetEmail(app, c.Request.Context(), serviceToken, user.ID); err != nil {
			internal.RespondInternalError(c, app, err, "Failed to trigger password reset email in Keycloak")
			return
		}

		app.InfoLog.Printf("Password reset email sent successfully to user %s (%s)\n", user.Username, input.Email)
		c.JSON(http.StatusOK, gin.H{"message": "Password reset email sent successfully"})
	}
}

func LoginHandler(app *internal.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input services.LoginInput
		if err := c.ShouldBindJSON(&input); err != nil {
			internal.RespondValidationError(c, internal.FormatValidationErrors(err))
			return
		}

		tokenData, err := services.LoginUser(app, c.Request.Context(), input.Username, input.Password)
		if err != nil {
			app.ErrorLog.Printf("Login failed for user %s: %v\n", input.Username, err)
			internal.RespondClientError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid username or password")
			return
		}

		// Set production-grade HttpOnly cookies
		setAuthCookies(c, app, tokenData.AccessToken, tokenData.RefreshToken, tokenData.ExpiresIn)

		app.InfoLog.Printf("User %s logged in successfully\n", input.Username)
		c.JSON(http.StatusOK, tokenData)
	}
}

func RefreshTokenHandler(app *internal.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		var refreshToken string
		var input services.RefreshTokenInput

		// Accept refresh token from JSON body or HttpOnly cookie
		if err := c.ShouldBindJSON(&input); err == nil && input.RefreshToken != "" {
			refreshToken = input.RefreshToken
		} else if cookieToken, err := c.Cookie("refresh_token"); err == nil && cookieToken != "" {
			refreshToken = cookieToken
		}

		if refreshToken == "" {
			internal.RespondClientError(c, http.StatusBadRequest, "MISSING_REFRESH_TOKEN", "Refresh token is missing from request body and cookies")
			return
		}

		tokenData, err := services.RefreshToken(app, c.Request.Context(), refreshToken)
		if err != nil {
			app.ErrorLog.Printf("Token refresh failed: %v\n", err)
			internal.RespondClientError(c, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Invalid or expired refresh token")
			return
		}

		// Update HttpOnly cookies with refreshed tokens
		setAuthCookies(c, app, tokenData.AccessToken, tokenData.RefreshToken, tokenData.ExpiresIn)

		c.JSON(http.StatusOK, tokenData)
	}
}

func LogoutHandler(app *internal.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		var refreshToken string
		var input services.LogoutInput

		// Accept refresh token from JSON body or HttpOnly cookie
		if err := c.ShouldBindJSON(&input); err == nil && input.RefreshToken != "" {
			refreshToken = input.RefreshToken
		} else if cookieToken, err := c.Cookie("refresh_token"); err == nil && cookieToken != "" {
			refreshToken = cookieToken
		}

		if refreshToken != "" {
			if err := services.LogoutUser(app, c.Request.Context(), refreshToken); err != nil {
				app.ErrorLog.Printf("Warning: Keycloak session logout failed: %v\n", err)
			}
		}

		// Delete authentication cookies from browser
		clearAuthCookies(c, app)

		app.InfoLog.Println("User logged out successfully")
		c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
	}
}

func GoogleLoginHandler(app *internal.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		redirectURI := c.Query("redirect_uri")
		if redirectURI == "" {
			scheme := "http"
			if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
				scheme = "https"
			}
			host := c.Request.Host
			if host == "" {
				host = "localhost:3000"
			}
			redirectURI = fmt.Sprintf("%s://%s/api/auth/callback", scheme, host)
		}

		targetFrontendURL := c.Query("redirect_to")
		if targetFrontendURL != "" && !validateRedirectURL(app, targetFrontendURL) {
			internal.RespondClientError(c, http.StatusBadRequest, "INVALID_REDIRECT_URL", "The provided redirect URL is not in the allowed origins whitelist")
			return
		}

		// Generate secure CSRF state and store in HttpOnly cookie
		state, err := generateOAuthState(c, app, targetFrontendURL)
		if err != nil {
			internal.RespondInternalError(c, app, err, "Failed to generate OAuth state")
			return
		}

		authURL := fmt.Sprintf(
			"%s/realms/%s/protocol/openid-connect/auth?client_id=%s&response_type=code&scope=openid%%20email%%20profile&redirect_uri=%s&kc_idp_hint=google&state=%s",
			app.Config.KeycloakPublicURL,
			app.Config.KeycloakRealm,
			app.Config.KeycloakClientID,
			url.QueryEscape(redirectURI),
			url.QueryEscape(state),
		)

		accept := c.GetHeader("Accept")
		isBrowser := strings.Contains(accept, "text/html")

		// Return JSON URL if requested via format=json OR if called from API testers (Postman/Bruno)
		if c.Query("format") == "json" || (!isBrowser && strings.Contains(accept, "application/json")) {
			c.JSON(http.StatusOK, gin.H{"url": authURL})
			return
		}

		c.Redirect(http.StatusTemporaryRedirect, authURL)
	}
}
