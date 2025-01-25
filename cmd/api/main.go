package main

import (
	"context"
	"log"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/auth"
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
		apiURL:    env.GetString("API_URL", "localhost:8080"),
		clientURL: env.GetString("CLIENT_URL", "http://localhost:3000"),
		addr:      env.GetString("ADDR", ":8080"),
		env:       env.GetString("ENV", "development"),
		authConfig: AuthConfig{
			AccessSecretKey:  env.GetString("ACCESS_SECRET_KEY", ""),
			RefreshSecretKey: env.GetString("REFRESH_SECRET_KEY", ""),
			AccessTokenTTL:   env.GetDuration("ACCESS_TOKEN_TTL", time.Minute*5),
			RefreshTokenTTL:  env.GetDuration("REFRESH_TOKEN_TLL", time.Hour*1),
			RememberMeTTL:    env.GetDuration("REMEMBER_ME_TTL", time.Hour*24*30),
		},
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
				isSandbox:       env.GetBool("MAIL_TRAP_SANDBOX_ENABLED", true),
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
	authToken, err := auth.NewPasetoToken(cfg.authConfig.AccessSecretKey, cfg.authConfig.RefreshSecretKey)

	if err != nil {
		logger.Panic(err)
	}
	app := &application{
		cfg:             cfg,
		logger:          logger,
		store:           store,
		cacheStore:      cacheStore,
		taskDistributor: taskDistributor,
		authToken:       authToken,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.background(func() {
		mailClient := mailer.NewMailTrapClient(
			cfg.mailConfig.mailTrap.fromEmail,
			cfg.mailConfig.mailTrap.smtpAddr,
			cfg.mailConfig.mailTrap.smtpSandboxAddr,
			cfg.mailConfig.mailTrap.username,
			cfg.mailConfig.mailTrap.password,
			cfg.mailConfig.mailTrap.smtpPort,
			cfg.mailConfig.mailTrap.isSandbox,
			logger,
		)
		app.runTaskProcessor(redisOpts, ctx, store, cacheStore, mailClient)
	})
	err = app.serve()

	if err != nil {
		log.Panic(err)
	}
}

func (app *application) runTaskProcessor(redisOpt asynq.RedisClientOpt, ctx context.Context, store *store.Storage, cacheStore *cache.Storage, mailClient mailer.Client) {
	taskProcessor := worker.NewRedisTaskProcessor(redisOpt, store, cacheStore, mailClient)

	app.logger.Info("start task processor")

	go func() {
		<-ctx.Done()
		taskProcessor.Close()
	}()

	err := taskProcessor.Start()

	if err != nil {
		app.logger.Fatalw("failed to start task processor", "err", err.Error())
	}
}
