package main

import (
	"errors"
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/validator"
)

type ResponseErrorCode string

const (
	ErrorCodeBadRequest          ResponseErrorCode = "bad_request"
	ErrorCodeRequired2FA         ResponseErrorCode = "required_2fa_code"
	ErrorCodeUnauthorized        ResponseErrorCode = "unauthorized"
	ErrorCodeForbidden           ResponseErrorCode = "forbidden"
	ErrorCodeNotFound            ResponseErrorCode = "not_found"
	ErrorCodeConflict            ResponseErrorCode = "conflict"
	ErrorCodeInvalidCredentials  ResponseErrorCode = "invalid_credentials"
	ErrorCodeInternalServerError ResponseErrorCode = "internal_server_error"
)

func (app *application) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.logger.Warnf("bad request error", "method", r.Method, "path", r.URL.Path, "error", err)

	var validationErrors *validator.ValidationErrors

	if errors.As(err, &validationErrors) {
		app.errorResponse(w, http.StatusBadRequest, validationErrors.FieldErrors(), envelope{"code": ErrorCodeBadRequest})
		return
	}
	app.errorResponse(w, http.StatusBadRequest, err.Error(), envelope{"code": ErrorCodeBadRequest})
}

func (app *application) required2faCodeResponse(w http.ResponseWriter, r *http.Request, token string) {
	app.logger.Warnf("required 2fa error", "method", r.Method, "path", r.URL.Path, "error")

	app.errorResponse(w, http.StatusForbidden, "2FA code is required", envelope{
		"code":  ErrorCodeRequired2FA,
		"token": token,
	})
}

func (app *application) unauthorizedResponse(w http.ResponseWriter, r *http.Request, message string) {
	app.logger.Warnf(
		"unauthorized access",
		"method", r.Method,
		"path", r.URL.Path,
	)

	app.errorResponse(w, http.StatusUnauthorized, message, envelope{"code": ErrorCodeUnauthorized})
}

func (app *application) conflictResponse(w http.ResponseWriter, r *http.Request, message string) {
	app.logger.Warnf(
		"conflict error",
		"method", r.Method,
		"path", r.URL.Path,
		"error", message,
	)

	app.errorResponse(w, http.StatusConflict, message, envelope{"code": ErrorCodeConflict})
}

func (app *application) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.logger.Errorw("internal server error", "method", r.Method, "path", r.URL.Path, "error", err)

	message := "the server encountered a problem and could not process your request"
	app.errorResponse(w, http.StatusInternalServerError, message, envelope{"code": ErrorCodeInternalServerError})
}

func (app *application) forbiddenResponse(w http.ResponseWriter, r *http.Request, details ...string) {
	app.logger.Errorw("forbidden access attempt",
		"method", r.Method,
		"path", r.URL.Path,
	)

	message := "you do not have permission to access this resource"
	if len(details) > 0 && details[0] != "" {
		message = details[0]
	}

	app.errorResponse(w, http.StatusForbidden, message, envelope{"code": ErrorCodeForbidden})
}

func (app *application) notFoundResponse(w http.ResponseWriter, r *http.Request, details ...string) {
	app.logger.Errorw("not found attempt",
		"method", r.Method,
		"path", r.URL.Path,
	)

	message := "the requested resource could not be found"
	if len(details) > 0 && details[0] != "" {
		message = details[0]
	}

	app.errorResponse(w, http.StatusNotFound, message, envelope{"code": ErrorCodeNotFound})
}

func (app *application) invalidCredentialsResponse(w http.ResponseWriter, r *http.Request) {
	app.logger.Infow("failed sign-in attempt", "method", r.Method, "path", r.URL.Path)

	message := "invalid credentials: incorrect email or password"
	app.errorResponse(w, http.StatusUnauthorized, message, envelope{"code": ErrorCodeInvalidCredentials})
}

func (app *application) errorResponse(w http.ResponseWriter, status int, message any, info ...envelope) {
	error := envelope{
		"message": message,
	}

	env := envelope{
		"status": "error",
		"error":  error,
	}

	if len(info) == 1 && len(info[0]) > 0 {
		for key, value := range info[0] {
			error[key] = value
		}
	}

	err := app.writeJSON(w, status, env, nil)
	if err != nil {
		app.logger.Info("Failed to write JSON response:", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (app *application) successResponse(w http.ResponseWriter, status int, data any) {
	env := envelope{
		"status": "success",
		"data":   data,
	}

	err := app.writeJSON(w, status, env, nil)
	if err != nil {
		app.logger.Info("Failed to write JSON response:", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
