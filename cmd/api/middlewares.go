package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/justinas/nosurf"
)

// Define a custom type for context keys to avoid collisions
type contextKey string

const authContextKey contextKey = "auth"

// AuthMiddleware authenticates requests using a Bearer token or cookie
func (app *application) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract the token from the Authorization header or cookie
		token := extractToken(r, app)
		if token == "" {
			// Add the user to the request context
			ctx := context.WithValue(r.Context(), authContextKey, &AuthInfo{
				User:        store.AnonymousUser,
				IsAnonymous: true,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Validate the token
		payload, err := app.authToken.ValidateAccessToken(token)
		if err != nil || payload == nil {
			app.unauthorizedResponse(w, r, "invalid or missing authentication token")
			return
		}

		// Verify the token payload
		if err := payload.Valid(); err != nil {
			app.unauthorizedResponse(w, r, "invalid authentication token")
			return
		}

		// Fetch the user associated with the token
		user, err := app.getUser(r.Context(), payload.UserID)

		if err != nil {
			app.unauthorizedResponse(w, r, "invalid authentication token")
			return
		}

		// After loading user
		if user.Role == store.AdminRole {
			adminUser, err := app.store.Users.GetAdminUserByID(r.Context(), user.ID)
			if err != nil {
				app.serverErrorResponse(w, r, err)
				return
			}
			user.AdminUser = adminUser
		}

		// Add the user to the request context
		ctx := context.WithValue(r.Context(), authContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractToken extracts the token from the Authorization header or cookie
func extractToken(r *http.Request, app *application) string {
	// Try to get the token from the Authorization header
	tokenHeader := r.Header.Get("Authorization")
	if tokenHeader != "" {
		// Check if the header is in the format "Bearer <token>"
		parts := strings.Split(tokenHeader, " ")
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
	}

	// Fall back to getting the token from the cookie
	return app.getAccessCookie(r)
}

// getUserFromCtx retrieves the authenticated user from the request context
func getUserFromCtx(r *http.Request) *AuthInfo {
	user, ok := r.Context().Value(authContextKey).(*AuthInfo)
	if !ok {
		panic("user context middleware not ran or functioning properly")
	}
	return user
}

func (app *application) requireAuthenicatedUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := getUserFromCtx(r)

		if user.User.IsAnonymous() {
			app.unauthorizedResponse(w, r, "you must be authenticated to access this resource")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func LoadCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if request is from a browser
		isBrowser := isBrowserRequest(r)

		// If not a browser request, skip CSRF
		if !isBrowser {
			next.ServeHTTP(w, r)
			return
		}

		// For browser requests, only apply CSRF to form submissions
		contentType := r.Header.Get("Content-Type")
		isFormData := strings.Contains(contentType, "multipart/form-data") ||
			strings.Contains(contentType, "application/x-www-form-urlencoded")

		if isFormData {
			csrfHandler := nosurf.New(next)
			csrfHandler.ServeHTTP(w, r)
			return
		}

		// For all other requests, proceed without CSRF
		next.ServeHTTP(w, r)
	})
}

// Helper function to determine if request is from a browser
func isBrowserRequest(r *http.Request) bool {
	// Check User-Agent
	userAgent := r.Header.Get("User-Agent")

	// Common browser indicators
	browserIndicators := []string{
		"Mozilla",
		"Chrome",
		"Safari",
		"Firefox",
		"Edge",
		"Opera",
	}

	// Check for browser-like User-Agent
	hasBrowserUA := false
	for _, indicator := range browserIndicators {
		if strings.Contains(userAgent, indicator) {
			hasBrowserUA = true
			break
		}
	}

	// Check for common browser headers
	acceptHeader := r.Header.Get("Accept")
	hasAcceptHeader := strings.Contains(acceptHeader, "text/html")

	// Check if request accepts HTML
	secFetchSite := r.Header.Get("Sec-Fetch-Site")
	secFetchMode := r.Header.Get("Sec-Fetch-Mode")
	hasSecHeaders := secFetchSite != "" || secFetchMode != ""

	// Consider it a browser request if it has a browser-like User-Agent
	// and either accepts HTML or has Sec-Fetch headers
	return hasBrowserUA && (hasAcceptHeader || hasSecHeaders)
}
