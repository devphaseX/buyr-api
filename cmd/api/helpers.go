package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
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
