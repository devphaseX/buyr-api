package main

import "net/http"

func (app *application) getAccessCookie(r *http.Request) string {
	cookie, err := r.Cookie(app.cfg.authConfig.AccesssCookieName)

	if err != nil {
		return ""
	}

	return cookie.Value
}
