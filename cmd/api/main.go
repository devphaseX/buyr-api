package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/auth"
	"github.com/devphaseX/buyr-api.git/internal/db"
	"github.com/devphaseX/buyr-api.git/internal/env"
	"github.com/devphaseX/buyr-api.git/internal/fileobject"
	"github.com/devphaseX/buyr-api.git/internal/mailer"
	"github.com/devphaseX/buyr-api.git/internal/ratelimiter"
	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/devphaseX/buyr-api.git/internal/store/cache"
	"github.com/devphaseX/buyr-api.git/internal/totp.go"
	"github.com/devphaseX/buyr-api.git/internal/validator"
	"github.com/devphaseX/buyr-api.git/worker"
	"github.com/devphaseX/buyr-api.git/worker/scheduler"
	"github.com/go-playground/form/v4"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stripe/stripe-go/v81"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var validate = validator.New()

func main() {
	cfg := config{
		apiURL:    env.GetString("API_URL", "http://localhost:8080"),
		clientURL: env.GetString("CLIENT_URL", "http://localhost:3000"),
		addr:      env.GetString("ADDR", ":8080"),
		env:       env.GetString("ENV", "development"),
		authConfig: AuthConfig{
			AccessSecretKey:   env.GetString("ACCESS_SECRET_KEY", ""),
			RefreshSecretKey:  env.GetString("REFRESH_SECRET_KEY", ""),
			AccessTokenTTL:    env.GetDuration("ACCESS_TOKEN_TTL", time.Minute*5),
			RefreshTokenTTL:   env.GetDuration("REFRESH_TOKEN_TLL", time.Hour*1),
			RememberMeTTL:     env.GetDuration("REMEMBER_ME_TTL", time.Hour*24*30),
			AccesssCookieName: env.GetString("ACCESS_COOKIE_NAME", ""),
			RefreshCookiName:  env.GetString("REFRESH_COOKIE_NAME", ""),
			totpIssuerName:    env.GetString("TOTP_ISSUER_NAME", "buyr"),
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
		encryptConfig: encryptConfig{
			masterSecretKey: env.GetString("MASTER_SECRET_KEY", ""),
		},

		supabaseConfig: supabaseConfig{
			apiURL:                 env.GetString("SUPABASE_PROJECT_URL", ""),
			apiKey:                 env.GetString("SUPABASE_API_KEY", ""),
			profileImageBucketName: env.GetString("SUPABASE_PROFILE_BUCKET_NAME", ""),
		},

		stripe: stripeConfig{
			apiKey:        env.GetString("STRIPE_SECRET_KEY", ""),
			webhookSecret: env.GetString("STRIPE_WEBHOOK_SECRET", ""),
			successURL:    env.GetString("STRIPE_SUCCESS_URL", ""),
			cancelURL:     env.GetString("STRIPE_CANCEL_URL", ""),
		},

		googleOauthConfig: googleOauthConfig{
			clientId:     env.GetString("GOOGLE_CLIENT_ID", ""),
			clientSecret: env.GetString("GOOGLE_CLIENT_SECRET", ""),
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
	schedulerOpts := &asynq.SchedulerOpts{}
	taskScheduler := scheduler.NewAsyncTaskScheduler(redisOpts, schedulerOpts)

	taskScheduler.RegisterTasks()

	authToken, err := auth.NewPasetoToken(cfg.authConfig.AccessSecretKey, cfg.authConfig.RefreshSecretKey)
	totp := totp.New()
	if err != nil {
		logger.Panic(err)
	}

	formDecoder := form.NewDecoder()
	// fileobject, err := fileobject.NewSupabaseStorage(cfg.supabaseConfig.apiURL, cfg.supabaseConfig.apiKey)

	fileobject := &fileobject.FileSystemStorage{
		BasePath: "./static",
	}

	stripe.Key = cfg.stripe.apiKey
	// For sample support and debugging, not required for production:
	stripe.SetAppInfo(&stripe.AppInfo{
		Name:    "Buyr",
		Version: "0.0.1",
		URL:     fmt.Sprintf("%s/webhook/stripe", cfg.apiURL),
	})

	oauthConfig := &oauth2.Config{
		ClientID:     cfg.googleOauthConfig.clientId,
		ClientSecret: cfg.googleOauthConfig.clientSecret,
		RedirectURL:  fmt.Sprintf("%s/v1/auth/google/callback", cfg.apiURL),
		Scopes:       []string{"email", "profile"},
		Endpoint:     google.Endpoint,
	}

	if err != nil {
		logger.Panic(err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.redisCfg.addr,
		DB:   cfg.redisCfg.db,
	})

	app := &application{
		cfg:             cfg,
		totp:            totp,
		logger:          logger,
		store:           store,
		cacheStore:      cacheStore,
		formDecoder:     formDecoder,
		fileobject:      fileobject,
		googleOauth:     oauthConfig,
		taskDistributor: taskDistributor,
		authToken:       authToken,
	}

	rateLimitService, err := ratelimiter.NewRateLimiterService(redisClient, ratelimiter.WithLimitReachedHandler(func(w http.ResponseWriter, r *http.Request) {
		app.rateLimitExceededResponse(w)
	}))

	if err != nil {
		logger.Panic(err)
	}

	app.rateLimitService = rateLimitService

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
		app.runTaskProcessor(redisOpts, taskDistributor, ctx, store, cacheStore, mailClient)
	})

	go app.background(func() {
		go func() {
			<-ctx.Done()
			taskScheduler.Close()
		}()

		taskScheduler.Run()
	})

	err = app.serve()

	if err != nil {
		log.Panic(err)
	}
}

func (app *application) runTaskProcessor(
	redisOpt asynq.RedisClientOpt,
	taskDistributor worker.TaskDistributor,
	ctx context.Context,
	store *store.Storage,
	cacheStore *cache.Storage,
	mailClient mailer.Client,
) {

	cronTaskProcessor := scheduler.NewAsyncTaskProcessor(redisOpt, store)
	taskProcessor := worker.NewRedisTaskProcessor(redisOpt, cronTaskProcessor, taskDistributor, store, cacheStore, mailClient)

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
