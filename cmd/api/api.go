package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type application struct {
	cfg config
}

type config struct {
	addr string
	env  string
}

func (app *application) routes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	return r
}

func (app *application) serve() error {
	srv := &http.Server{
		Addr:    app.cfg.addr,
		Handler: app.routes(),
	}

	err := srv.ListenAndServe()

	shutdownError := make(chan error)

	go func() {

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		s := <-quit

		fmt.Println("caught signal signal ", s.String())

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
		defer cancel()

		err := srv.Shutdown(ctx)

		if err != nil {
			shutdownError <- err
		}
	}()

	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdownError
	if err != nil {
		return err
	}

	return nil
}
