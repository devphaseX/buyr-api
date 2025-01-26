package main

import (
	"errors"
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/validator"
)

func (app *application) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.logger.Warnf("bad request error", "method", r.Method, "path", r.URL.Path, "error", err)

	var validationErrors *validator.ValidationErrors

	if errors.As(err, &validationErrors) {
		app.errorResponse(w, http.StatusBadRequest, validationErrors.FieldErrors())
		return
	}
	app.errorResponse(w, http.StatusBadRequest, err.Error())
}

func (app *application) unauthorizedResponse(w http.ResponseWriter, r *http.Request, message string) {
	// Log the unauthorized access attempt
	app.logger.Warnf(
		"unauthorized access",
		"method", r.Method,
		"path", r.URL.Path,
	)

	// Return a 401 Unauthorized or 403 Forbidden response
	status := http.StatusUnauthorized
	// Send the error response
	app.errorResponse(w, status, message)
}

func (app *application) conflictResponse(w http.ResponseWriter, r *http.Request, message string) {
	// Log the conflict error
	app.logger.Warnf(
		"conflict error",
		"method", r.Method,
		"path", r.URL.Path,
		"error", message,
	)

	// Return a 409 Conflict response
	status := http.StatusConflict

	// Send the error response
	app.errorResponse(w, status, message)
}

func (app *application) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.logger.Errorw("interal server error", "method", r.Method, "path", r.URL.Path, "error", err)

	message := "the server encountered a problem and could not process your request"
	app.errorResponse(w, http.StatusInternalServerError, message)
}

func (app *application) forbiddenResponse(w http.ResponseWriter, r *http.Request, details ...string) {
	// Log the forbidden error
	app.logger.Errorw("forbidden access attempt",
		"method", r.Method,
		"path", r.URL.Path,
	)

	// Determine the message to use
	message := "you do not have permission to access this resource"
	if len(details) > 0 && details[0] != "" {
		message = details[0]
	}

	// Send the forbidden response
	app.errorResponse(w, http.StatusForbidden, message)
}

func (app *application) invalidCredentialsResponse(w http.ResponseWriter, r *http.Request) {
	// Log the failed sign-in attempt
	app.logger.Infow("failed sign-in attempt", "method", r.Method, "path", r.URL.Path)

	// Define the error message
	message := "invalid credentials: incorrect email or password"

	// Send the error response
	app.errorResponse(w, http.StatusUnauthorized, message)
}

func (app *application) errorResponse(w http.ResponseWriter, status int, message any) {
	// Create a structured response envelope
	env := envelope{
		"status": "error", // Indicates that the request failed
		"error":  message, // Provides the error message or details
	}

	// Write the JSON response
	err := app.writeJSON(w, status, env, nil)
	if err != nil {
		// If there's an error writing the JSON, log it and return a 500 Internal Server Error
		app.logger.Info("Failed to write JSON response:", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (app *application) successResponse(w http.ResponseWriter, status int, data any) {
	// Create a structured response envelope
	env := envelope{
		"status": "success", // Indicates that the request was successful
		"data":   data,      // Contains the success payload
	}

	// Write the JSON response
	err := app.writeJSON(w, status, env, nil)
	if err != nil {
		// If there's an error writing the JSON, log it and return a 500 Internal Server Error
		app.logger.Info("Failed to write JSON response:", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
