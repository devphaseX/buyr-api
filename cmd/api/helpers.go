package main

import (
	"errors"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/encrypt"
	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/form/v4"
)

func (app *application) readStringID(r *http.Request, param string) string {
	return chi.URLParam(r, param)
}

func (app *application) background(fn func()) {
	// Increment the WaitGroup counter.
	app.wg.Add(1)
	// Launch a background goroutine.
	go func() {
		// Use defer to decrement the WaitGroup counter before the goroutine returns.
		defer app.wg.Done()
		// Recover any panic.
		defer func() {
			if err := recover(); err != nil {
				// app.logger.PrintError(fmt.Errorf("%s", err), nil)
			}
		}()
		// Execute the arbitrary function that we passed as the parameter.
		fn()
	}()
}

func (app *application) setAuthCookiesAndRespond(
	w http.ResponseWriter,
	accessToken string,
	accessTokenExpiry time.Duration,
	newRefreshToken string,
	rememberPeriod time.Duration,
) {
	// Set the access token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     app.cfg.authConfig.AccesssCookieName,
		Value:    accessToken,
		Expires:  time.Now().Add(accessTokenExpiry),
		Path:     "/",                     // Cookie is accessible across the entire site
		HttpOnly: true,                    // Prevent JavaScript access to the cookie
		Secure:   true,                    // Ensure the cookie is only sent over HTTPS
		SameSite: http.SameSiteStrictMode, // Prevent cross-site request forgery (CSRF)
	})

	// Conditionally set the refresh token cookie
	if newRefreshToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     app.cfg.authConfig.RefreshCookiName,
			Value:    newRefreshToken,
			Expires:  time.Now().Add(rememberPeriod),
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		})
	}

	// Create the response envelope
	response := envelope{
		"access_token":        accessToken,
		"access_token_expiry": time.Now().Add(accessTokenExpiry), // Unix timestamp for expiry
	}

	// Conditionally add refresh token fields to the response
	if newRefreshToken != "" {
		response["refresh_token"] = newRefreshToken
		response["refresh_token_expiry"] = time.Now().Add(rememberPeriod)
	}

	// Send the success response
	app.successResponse(w, http.StatusOK, response)
}

func (app *application) withPasswordAccess(r *http.Request, password string) bool {
	user := getUserFromCtx(r)

	match, err := user.Password.Matches(password)

	if err != nil {
		app.logger.Panic(err)
	}

	return match
}

func (app *application) createUserSessionAndSetCookies(w http.ResponseWriter, r *http.Request, user *store.User, RememberMe bool) {
	var (
		sessionExpiry     = app.cfg.authConfig.RefreshTokenTTL
		accessTokenExpiry = app.cfg.authConfig.AccessTokenTTL
	)
	session := &store.Session{
		UserID:     user.ID,
		IP:         r.RemoteAddr,
		UserAgent:  r.UserAgent(),
		Version:    1,
		ExpiresAt:  time.Now().Add(sessionExpiry),
		RememberMe: RememberMe,
	}

	if RememberMe {
		sessionExpiry = app.cfg.authConfig.RememberMeTTL
		session.ExpiresAt = time.Now().Add(sessionExpiry)
		session.MaxRenewalDuration = time.Now().AddDate(0, 6, 0).Unix() //6 months
	}

	err := app.store.Sessions.Create(r.Context(), session)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	accessToken, err := app.authToken.GenerateAccessToken(user.ID, session.ID, app.cfg.authConfig.AccessTokenTTL)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	refreshToken, err := app.authToken.GenerateRefreshToken(session.ID, session.Version, sessionExpiry)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.setAuthCookiesAndRespond(w,
		accessToken,
		accessTokenExpiry,
		refreshToken,
		sessionExpiry,
	)
}

func (app *application) decodeForm(r *http.Request, dst any, maxMemory int64) error {
	err := r.ParseMultipartForm(maxMemory)

	if err != nil {
		return err
	}

	err = app.formDecoder.Decode(dst, r.PostForm)

	if err != nil {
		var invalidDecoderError *form.InvalidDecoderError
		if errors.As(err, &invalidDecoderError) {
			panic(err)
		}

		return err
	}

	return nil
}

func isImage(fileHeader *multipart.FileHeader) bool {
	// Get the file's MIME type
	mimeType := mime.TypeByExtension(filepath.Ext(fileHeader.Filename))

	// Check if the MIME type starts with "image/"
	return strings.HasPrefix(mimeType, "image/")
}

func (app *application) verifyTOTP(user *store.User, code string) (bool, error) {
	secret, err := encrypt.DecryptSecret(user.AuthSecret, app.cfg.encryptConfig.masterSecretKey)

	if err != nil {
		return false, err
	}

	return !app.totp.VerifyCode(secret, code), nil
}
