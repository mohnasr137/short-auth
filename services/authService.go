package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"auth/internal"
)

type RegisterInput struct {
	Username  string `json:"username" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
}

type ChangePasswordInput struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type ForgotPasswordInput struct {
	Email string `json:"email" binding:"required,email"`
}

type KeycloakUser struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Enabled   bool   `json:"enabled"`
}

type LoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RefreshTokenInput struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type LogoutInput struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type CallbackInput struct {
	Code        string `json:"code" binding:"required"`
	RedirectURI string `json:"redirect_uri" binding:"required"`
}

type KeycloakTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

var (
	serviceTokenMu        sync.RWMutex
	cachedServiceToken    string
	serviceTokenExpiresAt time.Time
)

func GetServiceToken(app *internal.Application, ctx context.Context) (string, error) {
	// 1. Check in-memory cache first
	serviceTokenMu.RLock()
	if cachedServiceToken != "" && time.Now().Before(serviceTokenExpiresAt) {
		token := cachedServiceToken
		serviceTokenMu.RUnlock()
		return token, nil
	}
	serviceTokenMu.RUnlock()

	// 2. Fetch fresh token from Keycloak
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", app.Config.KeycloakBaseURL, app.Config.KeycloakRealm)

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", app.Config.KeycloakClientID)
	form.Set("client_secret", app.Config.KeycloakClientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token error (status %d): %s", resp.StatusCode, string(body))
	}

	var res struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	// 3. Cache token in memory with safety buffer (60 seconds)
	if res.AccessToken != "" {
		ttlSeconds := res.ExpiresIn - 60
		if ttlSeconds <= 0 {
			ttlSeconds = res.ExpiresIn / 2
		}
		if ttlSeconds > 0 {
			serviceTokenMu.Lock()
			cachedServiceToken = res.AccessToken
			serviceTokenExpiresAt = time.Now().Add(time.Duration(ttlSeconds) * time.Second)
			serviceTokenMu.Unlock()
		}
	}

	return res.AccessToken, nil
}

func VerifyOldPassword(app *internal.Application, ctx context.Context, username, oldPassword string) error {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", app.Config.KeycloakBaseURL, app.Config.KeycloakRealm)

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", app.Config.KeycloakClientID)
	form.Set("client_secret", app.Config.KeycloakClientSecret)
	form.Set("username", username)
	form.Set("password", oldPassword)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid old password")
	}

	return nil
}

func ExchangeCodeForToken(app *internal.Application, ctx context.Context, code, redirectURI string) (*KeycloakTokenResponse, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", app.Config.KeycloakBaseURL, app.Config.KeycloakRealm)

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", app.Config.KeycloakClientID)
	form.Set("client_secret", app.Config.KeycloakClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to exchange code (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp KeycloakTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

func FindUserByEmail(app *internal.Application, ctx context.Context, serviceToken, email string) (*KeycloakUser, error) {
	searchURL := fmt.Sprintf("%s/admin/realms/%s/users?email=%s&exact=true", app.Config.KeycloakBaseURL, app.Config.KeycloakRealm, url.QueryEscape(email))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+serviceToken)

	resp, err := app.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to search user (status %d): %s", resp.StatusCode, string(body))
	}

	var users []KeycloakUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return nil, nil
	}

	return &users[0], nil
}

func SendPasswordResetEmail(app *internal.Application, ctx context.Context, serviceToken, userID string) error {
	actionsURL := fmt.Sprintf("%s/admin/realms/%s/users/%s/execute-actions-email", app.Config.KeycloakBaseURL, app.Config.KeycloakRealm, userID)

	actions := []string{"UPDATE_PASSWORD"}
	payload, err := json.Marshal(actions)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", actionsURL, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+serviceToken)

	resp, err := app.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("keycloak reset email failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func LoginUser(app *internal.Application, ctx context.Context, username, password string) (*KeycloakTokenResponse, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", app.Config.KeycloakBaseURL, app.Config.KeycloakRealm)

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", app.Config.KeycloakClientID)
	form.Set("client_secret", app.Config.KeycloakClientSecret)
	form.Set("username", username)
	form.Set("password", password)
	form.Set("scope", "openid profile email basic")

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("login failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp KeycloakTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

func RefreshToken(app *internal.Application, ctx context.Context, refreshToken string) (*KeycloakTokenResponse, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", app.Config.KeycloakBaseURL, app.Config.KeycloakRealm)

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", app.Config.KeycloakClientID)
	form.Set("client_secret", app.Config.KeycloakClientSecret)
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp KeycloakTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

func LogoutUser(app *internal.Application, ctx context.Context, refreshToken string) error {
	logoutURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/logout", app.Config.KeycloakBaseURL, app.Config.KeycloakRealm)

	form := url.Values{}
	form.Set("client_id", app.Config.KeycloakClientID)
	form.Set("client_secret", app.Config.KeycloakClientSecret)
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", logoutURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("logout failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
