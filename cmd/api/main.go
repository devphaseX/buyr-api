package main

import (
	"log"

	"github.com/devphaseX/buyr-api.git/internal/env"
	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/devphaseX/buyr-api.git/internal/validator"
	"go.uber.org/zap"
)

var validate = validator.New()

func main() {
	cfg := config{
		apiURL: env.GetString("API_URL", "localhost:8080"),
		addr:   env.GetString("ADDR", ":8080"),
		env:    env.GetString("ENV", "development"),
		db: dbConfig{
			dsn:          env.GetString("DB_ADDR", "postgres://mingle:adminpassword@localhost/mingle?sslmode=disable"),
			maxOpenConns: env.GetInt("DB_MAX_OPEN_CONNS", 30),
			maxIdleConns: env.GetInt("DB_MAX_IDLE_CONNS", 30),
			maxIdleTime:  env.GetString("DB_MAX_IDLE_TIME", "15m"),
		},
	}

	logger := zap.Must(zap.NewProduction()).Sugar()
	defer logger.Sync()

	db, err := openDB(cfg)

	if err != nil {
		logger.Panic(err)
	}

	logger.Info("database connection pool established")

	store := store.NewStorage(db)
	app := &application{
		cfg:    cfg,
		logger: logger,
		store:  store,
	}

	err = app.serve()

	if err != nil {
		log.Panic(err)
	}
}
