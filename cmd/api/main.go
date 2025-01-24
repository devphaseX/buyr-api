package main

import (
	"log"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/db"
	"github.com/devphaseX/buyr-api.git/internal/env"
	"github.com/devphaseX/buyr-api.git/internal/mailer"
	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/devphaseX/buyr-api.git/internal/store/cache"
	"github.com/devphaseX/buyr-api.git/internal/validator"
	"github.com/devphaseX/buyr-api.git/worker"
	"github.com/hibiken/asynq"
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

		redisCfg: redisConfig{
			addr:    env.GetString("REDIS_ADDR", "0.0.0.0:6379"),
			pw:      env.GetString("REDIS_PW", ""),
			db:      env.GetInt("REDIS_DB", 0),
			enabled: env.GetBool("REDIS_ENABLED", true),
		},

		mailConfig: mailConfig{
			exp: time.Hour * 24 * 3, //3 days
			mailTrap: mailTrapConfig{
				fromEmail:       env.GetString("MAIL_TRAP_FROM_EMAIL", ""),
				apiKey:          env.GetString("MAIL_TRAP_API_KEY", ""),
				smtpAddr:        env.GetString("MAIL_TRAP_SMTP_ADDR", ""),
				smtpSandboxAddr: env.GetString("MAIL_TRAP_SANDBOX_ADDR", "sandbox.smtp.mailtrap.io"),
				smtpPort:        env.GetInt("MAIL_TRAP_SMTP_PORT", 0),
				username:        env.GetString("MAIL_TRAP_USERNAME", ""),
				password:        env.GetString("MAIL_TRAP_PASSWORD", ""),
			},
		},
	}

	logger := zap.Must(zap.NewProduction()).Sugar()
	defer logger.Sync()

	db, err := db.New(cfg.db.dsn, cfg.db.maxOpenConns, cfg.db.maxIdleConns, cfg.db.maxIdleTime)

	if err != nil {
		logger.Panic(err)
	}

	logger.Info("database connection pool established")

	store := store.NewStorage(db)

	rdb := cache.NewRedisClient(cfg.redisCfg.addr, cfg.redisCfg.pw, cfg.redisCfg.db)
	cacheStore := cache.NewRedisStorage(rdb)

	redisOpts := asynq.RedisClientOpt{
		Addr: cfg.redisCfg.addr,
	}

	taskDistributor := worker.NewTaskDistributor(redisOpts)

	app := &application{
		cfg:             cfg,
		logger:          logger,
		store:           store,
		cacheStore:      cacheStore,
		taskDistributor: taskDistributor,
	}

	go app.background(func() {
		mailClient := mailer.NewMailTrapClient(
			cfg.mailConfig.mailTrap.fromEmail,
			cfg.mailConfig.mailTrap.smtpAddr,
			cfg.mailConfig.mailTrap.smtpSandboxAddr,
			cfg.mailConfig.mailTrap.username,
			cfg.mailConfig.mailTrap.password,
			cfg.mailConfig.mailTrap.smtpPort,
			logger,
		)
		app.runTaskProcessor(redisOpts, store, cacheStore, mailClient)
	})
	err = app.serve()

	if err != nil {
		log.Panic(err)
	}
}

func (app *application) runTaskProcessor(redisOpt asynq.RedisClientOpt, store *store.Storage, cacheStore *cache.Storage, mailClient mailer.Client) {
	taskProcessor := worker.NewRedisTaskProcessor(redisOpt, store, cacheStore, mailClient)

	app.logger.Info("start task processor")

	err := taskProcessor.Start()

	if err != nil {
		app.logger.Fatalw("failed to start task processor", "err", err.Error())
	}
}
