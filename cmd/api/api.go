package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/auth"
	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/devphaseX/buyr-api.git/internal/store/cache"
	"github.com/devphaseX/buyr-api.git/internal/totp.go"
	"github.com/devphaseX/buyr-api.git/worker"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

type application struct {
	cfg             config
	totp            totp.TOTP
	wg              sync.WaitGroup
	logger          *zap.SugaredLogger
	store           *store.Storage
	authToken       auth.AuthToken
	cacheStore      *cache.Storage
	taskDistributor worker.TaskDistributor
}

type config struct {
	addr          string
	env           string
	apiURL        string
	clientURL     string
	db            dbConfig
	redisCfg      redisConfig
	mailConfig    mailConfig
	authConfig    AuthConfig
	encryptConfig encryptConfig
}

type AuthConfig struct {
	AccessSecretKey   string
	RefreshSecretKey  string
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	RememberMeTTL     time.Duration
	AccesssCookieName string
	RefreshCookiName  string
	totpIssuerName    string
}

type encryptConfig struct {
	masterSecretKey string
}

type mailConfig struct {
	exp      time.Duration
	mailTrap mailTrapConfig
}

type mailTrapConfig struct {
	fromEmail       string
	smtpAddr        string
	smtpSandboxAddr string
	smtpPort        int
	apiKey          string
	username        string
	password        string
	isSandbox       bool
}

type dbConfig struct {
	dsn          string
	maxOpenConns int
	maxIdleConns int
	maxIdleTime  string
}

type redisConfig struct {
	addr    string
	pw      string
	db      int
	enabled bool
}

func (app *application) routes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Route("/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", app.registerNormalUser)
			r.Post("/sign-in", app.signIn)
			r.Post("/sign-in/2fa", app.verify2FA)
			r.Post("/refresh", app.refreshToken)
			r.Post("/forget-password", app.forgetPassword)
			r.Post("/reset-password/verify-email", app.confirmForgetPasswordToken)
			r.Post("/reset-password/2fa", app.verifyForgetPassword2fa)
			r.Post("/reset-password/change", app.resetPassword)

		})

		r.Route("/users", func(r chi.Router) {
			r.Patch("/activate/{token}", app.activateUser)
		})

		r.Route("/mfa", func(r chi.Router) {
			r.Use(app.AuthMiddleware)
			r.Get("/setup", app.setup2fa)
			r.Post("/verify", app.verify2faSetup)
		})
	})
	return r
}

func (app *application) serve() error {
	srv := &http.Server{
		Addr:    app.cfg.addr,
		Handler: app.routes(),
	}

	shutdownError := make(chan error)

	go func() {

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		s := <-quit

		app.logger.Infow("caught signal", "signal", s.String())

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
		defer cancel()

		err := srv.Shutdown(ctx)

		if err != nil {
			shutdownError <- err
		}
	}()

	app.logger.Infow("server has started", "addr", app.cfg.addr, "env", app.cfg.env)
	err := srv.ListenAndServe()

	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdownError
	if err != nil {
		return err
	}

	app.logger.Infow("server has stopped", "addr", app.cfg.addr, "env", app.cfg.env)
	return nil
}
