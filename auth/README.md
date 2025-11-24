# Authentication Package

This package provides OIDC (OpenID Connect) authentication with session-based cookies for the Depositor service, similar to the implementation in the Ingestor service.

## Features

- **OIDC Authentication**: Full OAuth2 Authorization Code Flow with PKCE
- **Session-based cookies**: Secure HttpOnly cookies with SameSite support
- **SciCat Integration**: Compatible with SciCat's authentication system
- **Flexible Configuration**: Environment variable-based configuration
- **Optional Authentication**: Can be disabled for development/testing

## Configuration

Authentication is configured through environment variables. See [../.env.example](../.env.example) for all available options.

### Required Environment Variables

When authentication is enabled (`DISABLE_AUTH=false`):

- `OIDC_ISSUER_URL`: The OIDC provider's issuer URL (e.g., Keycloak realm URL)
- `OIDC_CLIENT_ID`: The client ID registered with the OIDC provider
- `OIDC_REDIRECT_URL`: The callback URL (must match registered redirect URI)

### Optional Environment Variables

- `OIDC_CLIENT_SECRET`: Client secret (if using confidential client)
- `SESSION_DURATION`: Session duration in seconds (default: 3600)
- `SECURE_COOKIES`: Set to `true` for HTTPS environments (default: false)
- `FRONTEND_URL`: Frontend application URL for redirects
- `BACKEND_URL`: Backend API URL
- `DISABLE_AUTH`: Set to `true` to disable authentication

## Endpoints

### Authentication Endpoints

- `GET /auth/login` - Initiates OIDC login flow
- `GET /auth/callback` - Handles OIDC callback
- `GET /auth/logout` - Logs out user and clears session
- `GET /auth/userinfo` - Returns current user information

### Response Format

#### `/auth/userinfo` - When logged in:
```json
{
  "logged_in": true,
  "email": "user@example.com",
  "profile": "https://...",
  "subject": "user-id",
  "expires_at": "2024-01-01T12:00:00Z"
}
```

#### `/auth/userinfo` - When not logged in:
```json
{
  "logged_in": false
}
```

## Usage in Handlers

### Getting Access Token

Access tokens are automatically extracted from the session and made available in the Gin context:

```go
func MyHandler(c *gin.Context) {
    token, err := auth.GetAccessToken(c)
    if err != nil {
        // User is not authenticated or token not available
        c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
        return
    }

    // Use token for API calls
    // e.g., to OneDep or other services
}
```

### Optional vs Required Authentication

The current implementation uses **optional authentication** via `OptionalAuthMiddleware()`. This means:
- Endpoints work without authentication
- If a user is logged in, their access token is available in the context
- Handlers can check for authentication when needed

To require authentication for specific routes, use the `AuthMiddleware()`:

```go
// Require authentication for specific routes
protected := router.Group("/protected")
protected.Use(authHandler.AuthMiddleware())
{
    protected.POST("/secure-endpoint", SecureHandler)
}
```

## Integration with SciCat

This authentication package is designed to work seamlessly with SciCat's OIDC setup:

1. **Same OIDC Provider**: Uses the same Keycloak/OIDC provider as SciCat
2. **Session Cookies**: Uses HttpOnly, Secure cookies with appropriate SameSite settings
3. **Access Token Forwarding**: Extracts and stores access tokens for forwarding to other services

## Security Features

- **PKCE (Proof Key for Code Exchange)**: Protection against authorization code interception
- **State Parameter**: CSRF protection during OAuth flow
- **Nonce Validation**: ID token replay attack prevention
- **HttpOnly Cookies**: Prevents XSS attacks on session cookies
- **Secure Cookies**: HTTPS-only transmission when enabled
- **SameSite Cookies**: CSRF protection

## Development Mode

For local development without authentication:

```bash
export DISABLE_AUTH=true
```

This allows testing the API without setting up an OIDC provider.

## Example Setup with Keycloak

1. Create a new client in Keycloak:
   - Client ID: `depositor`
   - Client Protocol: `openid-connect`
   - Access Type: `confidential`
   - Valid Redirect URIs: `http://localhost:8080/auth/callback`

2. Set environment variables:
```bash
export OIDC_ISSUER_URL="http://localhost:8080/realms/scicat"
export OIDC_CLIENT_ID="depositor"
export OIDC_CLIENT_SECRET="your-client-secret"
export OIDC_REDIRECT_URL="http://localhost:8080/auth/callback"
export FRONTEND_URL="http://localhost:4200"
```

3. Start the service:
```bash
go run main.go
```

## Architecture

The authentication flow follows the standard OAuth2 Authorization Code Flow:

1. User clicks "Login" → redirected to `/auth/login`
2. `/auth/login` generates state, verifier, nonce → stores in session → redirects to OIDC provider
3. User authenticates at OIDC provider → redirected back to `/auth/callback`
4. `/auth/callback` validates state → exchanges code for tokens → stores user session
5. User is redirected to frontend application

Session data is stored in encrypted cookies managed by `gin-contrib/sessions`.

## Differences from Ingestor

While this implementation follows the same pattern as the Ingestor, there are some differences:

- **No Globus Integration**: Depositor doesn't need Globus authentication
- **Simplified Claims**: Doesn't parse complex Keycloak claims (roles, resource access)
- **Optional by Default**: Uses optional authentication middleware for all routes
- **Focused on Token Forwarding**: Primary goal is to obtain and forward access tokens to OneDep/EMPIAR

## Troubleshooting

### "auth session has expired or is invalid"
- The session cookie expired (default 5 minutes for auth flow)
- Clear browser cookies and try logging in again

### "login session has expired"
- User session expired (default 1 hour)
- Log in again through `/auth/login`

### "invalid state"
- CSRF protection triggered
- Possible cookie issues or session tampering
- Clear cookies and retry

### Cookies not being set
- Check `SECURE_COOKIES` setting matches your environment (HTTPS vs HTTP)
- Verify `ALLOW_ORIGINS` includes your frontend URL
- Ensure `AllowCredentials: true` in CORS configuration
