package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/encrypt"
	"github.com/devphaseX/buyr-api.git/internal/store"
	"golang.org/x/oauth2"
)

func (app *application) signInWithProvider(w http.ResponseWriter, r *http.Request) {
	state, err := encrypt.GenerateRandomString(32)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	url := app.googleOauth.AuthCodeURL(state)

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
		Secure:   app.cfg.env == "production",
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (app *application) googleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve the state from the cookie
	secretState, err := r.Cookie("oauth_state")
	if err != nil {
		app.badRequestResponse(w, r, errors.New("missing OAuth state cookie"))
		return
	}

	state := r.URL.Query().Get("state")
	if state != secretState.Value {
		app.badRequestResponse(w, r, errors.New("invalid OAuth state"))
		return
	}

	code := r.URL.Query().Get("code")
	token, err := app.googleOauth.Exchange(r.Context(), code)
	if err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("code exchange failed: %w", err))
		return
	}

	userData, err := app.fetchUserData(r.Context(), token)
	if err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to fetch user data: %v", err))
		return
	}

	user, err := app.store.Users.GetByEmail(r.Context(), userData.Email)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {

			var (
				names           = strings.Fields(userData.Name)
				emailVerifiedAt = time.Now()
			)

			normalUser := &store.NormalUser{
				FirstName: names[0],
				LastName:  names[1],
				User: store.User{
					Email:     userData.Email, // Use userData.Email
					Role:      store.UserRole,
					AvatarURL: userData.Image,
					IsActive:  true,
				},
			}

			if userData.VerifiedEmail {
				normalUser.User.EmailVerifiedAt = &emailVerifiedAt
			}

			err := app.store.Users.CreateNormalUser(r.Context(), normalUser)
			if err != nil {
				app.serverErrorResponse(w, r, fmt.Errorf("failed to create user: %v", err))
				return
			}

			user = &normalUser.User
		} else {
			app.serverErrorResponse(w, r, fmt.Errorf("failed to fetch user: %v", err))
			return
		}
	}

	account := &store.Account{
		UserID:            user.ID,
		Provider:          "google",
		ProviderAccountID: userData.ID,
		Type:              "oauth",
		AccessToken:       token.AccessToken,
		RefreshToken:      token.RefreshToken,
		TokenType:         token.TokenType,
		ExpiresAt:         token.Expiry.Unix(),
		Scope:             fmt.Sprintf("%s", token.Extra("scope")),
	}

	err = app.store.Users.UpsertAccount(r.Context(), account)
	if err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to upsert account: %v", err))
		return
	}

	if user.EmailVerifiedAt == nil {
		err = app.store.Users.SetUserAccountAsActivate(r.Context(), user)

		if err != nil {
			app.serverErrorResponse(w, r, fmt.Errorf("failed to upsert account: %v", err))
			return
		}
	}

	app.createUserSessionAndSetCookies(w, r, user, true)

}

type UserData struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	Image         string `json:"picture"`
	VerifiedEmail bool   `json:"verified_email"`
}

func (app *application) fetchUserData(ctx context.Context, token *oauth2.Token) (*UserData, error) {
	client := app.googleOauth.Client(ctx, token)

	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user data: %v", err)
	}
	defer resp.Body.Close()

	var userData UserData
	if err := json.NewDecoder(resp.Body).Decode(&userData); err != nil {
		return nil, fmt.Errorf("failed to parse user data: %v", err)
	}

	return &userData, nil
}
