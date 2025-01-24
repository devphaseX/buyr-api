package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *application) readStringID(r *http.Request, param string) string {
	return chi.URLParam(r, param)
}
