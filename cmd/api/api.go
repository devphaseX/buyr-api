package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/auth"
	"github.com/devphaseX/buyr-api.git/internal/fileobject"
	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/devphaseX/buyr-api.git/internal/store/cache"
	"github.com/devphaseX/buyr-api.git/internal/totp.go"
	"github.com/devphaseX/buyr-api.git/worker"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-playground/form/v4"
	"go.uber.org/zap"
)

type application struct {
	cfg             config
	totp            totp.TOTP
	wg              sync.WaitGroup
	logger          *zap.SugaredLogger
	store           *store.Storage
	authToken       auth.AuthToken
	fileobject      fileobject.FileObject
	formDecoder     *form.Decoder
	cacheStore      *cache.Storage
	taskDistributor worker.TaskDistributor
}

type config struct {
	addr           string
	env            string
	apiURL         string
	clientURL      string
	db             dbConfig
	redisCfg       redisConfig
	mailConfig     mailConfig
	authConfig     AuthConfig
	encryptConfig  encryptConfig
	supabaseConfig supabaseConfig
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

type supabaseConfig struct {
	apiURL                 string
	apiKey                 string
	profileImageBucketName string
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

	r.Use(app.AuthMiddleware)
	r.Use(app.loadCSRF)

	r.Route("/v1", func(r chi.Router) {
		r.Get("/csrf-token", app.getCSRFToken)
		r.Get("/categories", app.getPublicCategories)

		workDir, _ := os.Getwd()
		filesDir := http.Dir(filepath.Join(workDir, "static"))
		FileServer(r, "/static", filesDir)

		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", app.registerNormalUser)
			r.Post("/sign-in", app.signIn)
			r.Post("/sign-in/2fa", app.verifyLogin2FA)
			r.Post("/sign-in/recovery-code", app.verifyLogin2faRecoveryCode)
			r.Post("/refresh", app.refreshToken)
			r.Post("/forget-password", app.forgetPassword)
			r.Post("/reset-password/verify-email", app.confirmForgetPasswordToken)
			r.Post("/reset-password/2fa", app.verifyForgetPassword2fa)
			r.Post("/reset-password/recovery-code", app.verifyForgetPasswordRecoveryCode)
			r.Post("/reset-password/change", app.resetPassword)

		})

		r.Route("/users", func(r chi.Router) {
			r.Patch("/activate-account", app.activateUser)

			r.Group(func(r chi.Router) {
				r.Use(app.requireAuthenicatedUser)
				r.Get("/", app.getNormalUsers)
				r.Get("/current", app.getCurrentUser)

			})

		})

		r.Route("/files", func(r chi.Router) {
			r.Post("/image", app.uploadImage)
		})

		r.Route("/mfa", func(r chi.Router) {
			r.Use(app.requireAuthenicatedUser)
			r.With(app.CheckPermissions(RequireRoles(store.AdminRole, store.VendorRole))).Group(
				func(r chi.Router) {
					r.Get("/setup", app.setup2fa)
					r.Post("/verify", app.verify2faSetup)
					r.Post("/recovery-codes", app.viewRecoveryCodes)
					r.Patch("/recovery-codes/reset", app.resetRecoveryCodes)
				},
			)

		})

		r.Route("/products", func(r chi.Router) {
			r.Get("/", app.getProducts)

			r.Route("/{productID}", func(r chi.Router) {
				r.Get("/", app.getProduct)
				r.Route("/reviews", func(r chi.Router) {
					r.Get("/", app.getProductReviews)

					r.With(app.requireAuthenicatedUser).Group(func(r chi.Router) {
						r.With(app.CheckPermissions(RequireRoles(store.UserRole))).Post("/", app.createComment)
					})
				})
			})

			r.Group(func(r chi.Router) {
				r.Use(app.requireAuthenicatedUser)

				r.With(app.CheckPermissions(RequireRoles(store.VendorRole))).Group(func(r chi.Router) {
					r.Post("/", app.createProduct)
					r.Patch("/{id}/publish", app.publishProduct)
					r.Patch("/{id}/unpublish", app.unPublishProduct)
				})

				r.With(app.CheckPermissions(RequireLevels(store.AdminLevelManager))).Group(func(r chi.Router) {
					r.Patch("/{id}/approve", app.approveProduct)
					r.Patch("/{id}/reject", app.rejectProduct)
				})
			})

		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(app.requireAuthenicatedUser)
			r.Use(app.CheckPermissions(RequireRoles(store.AdminRole)))

			r.Route("/members", func(r chi.Router) {
				r.With(app.CheckPermissions(MinimumAdminLevel(store.AdminLevelSuper))).Post("/", app.createAdmin)
				r.Get("/", app.getAdminUsers)
			})

			r.Route("/users", func(r chi.Router) {
				r.Get("/", app.getNormalUsers)
			})

			r.Route("/vendors", func(r chi.Router) {
				r.Get("/", app.getVendorUsers)
				r.Post("/", app.createVendor)
			})

			r.Route("/categories", func(r chi.Router) {
				r.With(app.CheckPermissions(RequireLevels(store.AdminLevelSuper))).Post("/", app.createCategory)
				r.Get("/", app.getAdminCategoriesView)
				r.With(app.CheckPermissions(MinimumAdminLevel(store.AdminLevelManager))).Delete("/{id}", app.removeCategory)

				r.With(app.CheckPermissions(MinimumAdminLevel(store.AdminLevelManager))).Put("/{id}/visibility", app.setCategoryVisibility)
			})
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
