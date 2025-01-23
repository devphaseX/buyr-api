package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type application struct {
	cfg config
}

type config struct {
	addr string
	env  string
}

func (app *application) routes() http.Handler {
	mux := chi.NewRouter()
	return mux
}

func (app *application) serve() error {
	return nil
}
