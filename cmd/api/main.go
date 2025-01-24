package main

import (
	"log"

	"github.com/devphaseX/buyr-api.git/internal/env"
	"github.com/devphaseX/buyr-api.git/internal/validator"
	"go.uber.org/zap"
)

var validate = validator.New()

func main() {

	cfg := config{
		addr: env.GetString("ADDR", ":8080"),
		env:  env.GetString("ENV", "development"),
	}

	logger := zap.Must(zap.NewProduction()).Sugar()
	defer logger.Sync()

	app := &application{
		cfg:    cfg,
		logger: logger,
	}

	err := app.serve()

	if err != nil {
		log.Panic(err)
	}
}
