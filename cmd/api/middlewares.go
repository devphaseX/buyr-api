package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/devphaseX/buyr-api.git/internal/store"
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
			ctx := context.WithValue(r.Context(), authContextKey, store.AnonymousUser)
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
func getUserFromCtx(r *http.Request) *store.User {
	user, ok := r.Context().Value(authContextKey).(*store.User)
	if !ok {
		panic("user context middleware not ran or functioning properly")
	}
	return user
}

func (app *application) requireAuthenicatedUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := getUserFromCtx(r)

		if user.IsAnonymous() {
			app.unauthorizedResponse(w, r, "you must be authenticated to access this resource")
			return
		}

		next.ServeHTTP(w, r)
	})
}
