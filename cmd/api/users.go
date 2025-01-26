package main

import (
	"net/http"
)

func (app *application) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	userProfile, err := app.store.Users.FlattenUser(r.Context(), user)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusOK, envelope{
		"user": userProfile,
	})
}
