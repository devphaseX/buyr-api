package main

import (
	"log"

	"github.com/devphaseX/buyr-api.git/internal/env"
)

func main() {

	cfg := config{
		addr: env.GetString("ADDR", ":8080"),
		env:  env.GetString("ENV", "development"),
	}

	app := &application{
		cfg: cfg,
	}

	err := app.serve()

	if err != nil {
		log.Panic(err)
	}
}
