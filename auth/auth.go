package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

type AuthHandler struct {
	oauth2Config    *oauth2.Config
	oidcProvider    *oidc.Provider
	oidcVerifier    *oidc.IDTokenVerifier
	jwtKeyfunc      interface{}
	jwtSignMethods  []string
	sessionDuration uint
	secureCookies   bool
	frontendURL     string
	backendURL      string
}

func NewAuthHandler(config AuthConfig) (*AuthHandler, error) {
	ctx := context.Background()

	// Initialize OIDC provider
	provider, err := oidc.NewProvider(ctx, config.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	// Configure OAuth2
	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	// Configure OIDC verifier
	oidcVerifier := provider.Verifier(&oidc.Config{
		ClientID: config.ClientID,
	})

	return &AuthHandler{
		oauth2Config:    oauth2Config,
		oidcProvider:    provider,
		oidcVerifier:    oidcVerifier,
		sessionDuration: config.SessionDuration,
		secureCookies:   config.SecureCookies,
		frontendURL:     config.FrontendURL,
		backendURL:      config.BackendURL,
	}, nil
}

// GetLogin initiates the OAuth2 authorization code flow
func (a *AuthHandler) GetLogin(c *gin.Context) {
	authSession := sessions.DefaultMany(c, "auth")

	// Generate state and verifier for PKCE
	state, err := generateRandomString(16)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate state"})
		return
	}
	verifier := oauth2.GenerateVerifier()
	nonce, err := generateRandomString(32)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate nonce"})
		return
	}

	// Store in session
	sameSiteMode := http.SameSiteLaxMode
	if a.secureCookies || (c.Request.TLS != nil) {
		sameSiteMode = http.SameSiteNoneMode
	}
	authSession.Options(sessions.Options{
		HttpOnly: true,
		MaxAge:   300,
		Secure:   a.secureCookies || (c.Request.TLS != nil),
		SameSite: sameSiteMode,
	})
	authSession.Set("state", state)
	authSession.Set("verifier", verifier)
	authSession.Set("nonce", nonce)
	err = authSession.Save()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save session"})
		return
	}

	// Redirect to OIDC provider
	authURL := a.oauth2Config.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(verifier),
		oidc.Nonce(nonce),
	)
	c.Redirect(http.StatusFound, authURL)
}

// GetCallback handles the OAuth2 callback
func (a *AuthHandler) GetCallback(c *gin.Context) {
	authSession := sessions.DefaultMany(c, "auth")
	userSession := sessions.DefaultMany(c, "user")

	// Get auth values from session
	state, ok1 := authSession.Get("state").(string)
	verifier, ok2 := authSession.Get("verifier").(string)
	nonce, ok3 := authSession.Get("nonce").(string)
	if !(ok1 && ok2 && ok3) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "auth session has expired or is invalid"})
		return
	}

	// Delete auth session
	authSession.Delete("state")
	authSession.Delete("verifier")
	authSession.Delete("nonce")
	sameSiteMode := http.SameSiteLaxMode
	if a.secureCookies || (c.Request.TLS != nil) {
		sameSiteMode = http.SameSiteNoneMode
	}
	authSession.Options(sessions.Options{
		HttpOnly: true,
		Secure:   a.secureCookies || (c.Request.TLS != nil),
		SameSite: sameSiteMode,
		MaxAge:   -1,
	})
	err := authSession.Save()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Verify state (CSRF protection)
	if c.Query("state") != state {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid state"})
		return
	}

	// Exchange authorization code for token
	ctx := c.Request.Context()
	oauthToken, err := a.oauth2Config.Exchange(
		ctx,
		c.Query("code"),
		oauth2.VerifierOption(verifier),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("code exchange failed: %s", err.Error())})
		return
	}

	// Create token source
	tokenSource := a.oauth2Config.TokenSource(ctx, oauthToken)

	// Get user info
	userInfo, err := a.oidcProvider.UserInfo(ctx, tokenSource)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Verify ID token
	rawIDToken, ok := oauthToken.Extra("id_token").(string)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "'id_token' field was not found in oauth2 token"})
		return
	}
	idToken, err := a.oidcVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("idToken verification failed: %s", err.Error())})
		return
	}
	if idToken.Nonce != nonce {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nonce did not match"})
		return
	}

	// Set user session cookie
	userSession.Set("expires_at", time.Now().Add(time.Second*time.Duration(a.sessionDuration)).Format(time.RFC3339Nano))
	userSession.Set("email", userInfo.Email)
	userSession.Set("profile", userInfo.Profile)
	userSession.Set("subject", userInfo.Subject)
	userSession.Set("access_token", oauthToken.AccessToken)
	userSession.Options(sessions.Options{
		HttpOnly: true,
		MaxAge:   int(a.sessionDuration),
		Secure:   a.secureCookies || (c.Request.TLS != nil),
		SameSite: sameSiteMode,
	})
	err = userSession.Save()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("can't set user session: %s", err.Error())})
		return
	}

	// Redirect to frontend
	c.Redirect(http.StatusFound, a.frontendURL)
}

// GetLogout handles user logout
func (a *AuthHandler) GetLogout(c *gin.Context) {
	userSession := sessions.DefaultMany(c, "user")
	sameSiteMode := http.SameSiteLaxMode
	if a.secureCookies || (c.Request.TLS != nil) {
		sameSiteMode = http.SameSiteNoneMode
	}
	userSession.Options(sessions.Options{
		HttpOnly: true,
		Secure:   a.secureCookies || (c.Request.TLS != nil),
		SameSite: sameSiteMode,
		MaxAge:   -1,
	})
	err := userSession.Save()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logoutURI := a.oidcProvider.Endpoint().AuthURL + "/../logout"
	redirectURI := a.frontendURL

	c.Redirect(http.StatusFound, fmt.Sprintf("%s?post_logout_redirect_uri=%s&client_id=%s", logoutURI, redirectURI, a.oauth2Config.ClientID))
}

// GetUserinfo returns information about the logged-in user
func (a *AuthHandler) GetUserinfo(c *gin.Context) {
	userSession := sessions.DefaultMany(c, "user")
	expiresAtString, ok1 := userSession.Get("expires_at").(string)
	email, ok2 := userSession.Get("email").(string)
	profile, ok3 := userSession.Get("profile").(string)
	subject, ok4 := userSession.Get("subject").(string)

	if !(ok1 && ok2 && ok3 && ok4) {
		c.JSON(http.StatusOK, gin.H{"logged_in": false})
		return
	}

	expiresAt, err := time.Parse(time.RFC3339Nano, expiresAtString)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "can't parse expiry"})
		return
	}

	if expiresAt.Before(time.Now()) {
		c.JSON(http.StatusOK, gin.H{"logged_in": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logged_in": true,
		"email":     email,
		"profile":   profile,
		"subject":   subject,
		"expires_at": expiresAt,
	})
}

// AuthMiddleware verifies that the user is authenticated
func (a *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userSession := sessions.DefaultMany(c, "user")

		// Check expiry
		expiresAtString, ok := userSession.Get("expires_at").(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "login session has expired"})
			c.Abort()
			return
		}
		expiresAt, err := time.Parse(time.RFC3339Nano, expiresAtString)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "can't parse expire time"})
			c.Abort()
			return
		}
		if expiresAt.Before(time.Now()) {
			userSession.Options(sessions.Options{
				MaxAge: -1,
			})
			_ = userSession.Save()
			c.JSON(http.StatusUnauthorized, gin.H{"error": "login session has expired"})
			c.Abort()
			return
		}

		// Store access token in context for handlers to use
		accessToken, ok := userSession.Get("access_token").(string)
		if ok {
			c.Set("access_token", accessToken)
		}

		c.Next()
	}
}

// OptionalAuthMiddleware extracts token if available but doesn't require it
func (a *AuthHandler) OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userSession := sessions.DefaultMany(c, "user")

		// Check expiry
		expiresAtString, ok := userSession.Get("expires_at").(string)
		if !ok {
			c.Next()
			return
		}
		expiresAt, err := time.Parse(time.RFC3339Nano, expiresAtString)
		if err != nil {
			c.Next()
			return
		}
		if expiresAt.Before(time.Now()) {
			c.Next()
			return
		}

		// Store access token in context for handlers to use
		accessToken, ok := userSession.Get("access_token").(string)
		if ok {
			c.Set("access_token", accessToken)
		}

		c.Next()
	}
}

// GetAccessToken retrieves the access token from the gin context
func GetAccessToken(c *gin.Context) (string, error) {
	token, exists := c.Get("access_token")
	if !exists {
		return "", errors.New("access token not found in context")
	}
	tokenStr, ok := token.(string)
	if !ok {
		return "", errors.New("access token is not a string")
	}
	return tokenStr, nil
}

// ParseKeycloakClaims extracts custom claims from the JWT token
func ParseKeycloakClaims(c *gin.Context) (map[string]interface{}, error) {
	userSession := sessions.DefaultMany(c, "user")

	// For now, return basic user info from session
	// You can extend this to parse JWT claims if needed
	claims := make(map[string]interface{})

	if email, ok := userSession.Get("email").(string); ok {
		claims["email"] = email
	}
	if subject, ok := userSession.Get("subject").(string); ok {
		claims["sub"] = subject
	}

	return claims, nil
}
