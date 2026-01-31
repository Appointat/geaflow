package auth

import (
	"errors"
	"net/http"
)

// ValidateJWT simulates JWT validation and returns the userID
// In production, this should use a JWT library (like golang-jwt/jwt) to verify signature and expiration
func ValidateJWT(token string) (string, error) {
	if token == "" {
		return "", errors.New("missing auth_token")
	}

	// TODO: Implement real JWT validation
	// For now, we simply use the token as userID for testing
	if token == "valid-token-user-1" {
		return "user-1", nil
	}

	return "", errors.New("invalid token")
}

// AuthenticateHTTPRequest simulates HTTP proxy request authentication
// In production, this should check Authorization header or other API keys
func AuthenticateHTTPRequest(r *http.Request) (string, error) {
	// TODO: Implement real authentication
	// Example implementations (commented out):
	// apiKey := r.Header.Get("x-goog-api-key")
	// if apiKey == "secret-key-for-user-1" {
	//     return "user-1", nil
	// }
	// Or extract from request path: /proxy/user-1/...

	// For now, return a hardcoded user for testing
	return "user-1", nil
}
