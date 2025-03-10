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
	"github.com/devphaseX/buyr-api.git/internal/ratelimiter"
	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/devphaseX/buyr-api.git/internal/store/cache"
	"github.com/devphaseX/buyr-api.git/internal/totp.go"
	"github.com/devphaseX/buyr-api.git/worker"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-playground/form/v4"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type application struct {
	cfg              config
	totp             totp.TOTP
	wg               sync.WaitGroup
	logger           *zap.SugaredLogger
	store            *store.Storage
	authToken        auth.AuthToken
	googleOauth      *oauth2.Config
	rateLimitService *ratelimiter.RateLimiterService
	fileobject       fileobject.FileObject
	formDecoder      *form.Decoder
	cacheStore       *cache.Storage
	taskDistributor  worker.TaskDistributor
}

type config struct {
	addr              string
	env               string
	apiURL            string
	clientURL         string
	db                dbConfig
	redisCfg          redisConfig
	mailConfig        mailConfig
	authConfig        AuthConfig
	encryptConfig     encryptConfig
	supabaseConfig    supabaseConfig
	stripe            stripeConfig
	googleOauthConfig googleOauthConfig
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

type stripeConfig struct {
	apiKey        string
	successURL    string
	cancelURL     string
	webhookSecret string
}

type encryptConfig struct {
	masterSecretKey string
}

type mailConfig struct {
	exp      time.Duration
	mailTrap mailTrapConfig
}

type googleOauthConfig struct {
	clientId     string
	clientSecret string
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

	app.setRateLimit()

	r.Use(
		cors.Handler(cors.Options{
			AllowedOrigins:   []string{"https://*", "http://*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			ExposedHeaders:   []string{"Link", "Vary"},
			AllowCredentials: false,
			MaxAge:           300, // Maximum value not ignored by any of major browsers
		}))
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Use(app.AuthMiddleware)
	r.Use(app.loadCSRF)
	r.Use(app.rateLimitService.Middleware())

	r.Route("/v1", func(r chi.Router) {
		r.Get("/csrf-token", app.getCSRFToken)

		r.Route("/categories", func(r chi.Router) {
			r.Get("/", app.getPublicCategories)
		})

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

			r.Post("/google", app.signInWithProvider)
			r.Post("/google/callback", app.googleCallbackHandler)

		})

		r.Route("/users", func(r chi.Router) {

			r.Patch("/activate-account", app.activateUser)
			r.Patch("/email/verify-email", app.verifyEmailChange)

			r.Group(func(r chi.Router) {
				r.Use(app.requireAuthenicatedUser)
				r.Get("/", app.getNormalUsers)
				r.Get("/current", app.getCurrentUser)
				r.Patch("/change-password", app.changePassword)
				r.Patch("/change-password/2fa", app.verifyChangePassword2fa)

				r.Post("/email/initiate-change", app.initiateEmailChange)
				r.Post("/email/verify-2fa", app.verifyEmailChange2fa)
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

		r.Route("/carts", func(r chi.Router) {
			r.Use(app.requireAuthenicatedUser)
			r.With(app.CheckPermissions(RequireRoles(store.UserRole))).Group(func(r chi.Router) {

				r.Get("/", app.getCurrentUserCart)
				r.Get("/items", app.getGroupVendorCartItem)
				r.Get("/items/vendor", app.getVendorCartItem)
				r.Post("/checkout", app.handleCheckout)
				r.Post("/{orderID}/pay", app.initiatePayment)

				r.Post("/items", app.addCardItem)
				r.Get("/items/{cardItemID}", app.getCartItemByID)
				r.Delete("/items/{itemID}", app.removeCartItem)
				r.Patch("/items/{itemID}", app.setCartItemQuantity)

			})
		})

		r.Route("/orders", func(r chi.Router) {
			r.Use(app.requireAuthenicatedUser)
			r.With(app.CheckPermissions(RequireRoles(store.UserRole))).Group(func(r chi.Router) {
				r.Post("/webhook/stripe", app.handleStripeWebhook)
				r.Get("/", app.getUserViewOrderLists)
				r.Get("/{orderID}", app.getOrderForUserByID)
			})
		})

		r.Route("/addresses", func(r chi.Router) {
			r.Get("/", app.getUserAddresses)
			r.Post("/", app.createUserAddress)
			r.Get("/{addressID}", app.getUserAddressByID)
			r.Get("/default", app.setDefaultAddress)
		})

		r.Route("/option-types", func(r chi.Router) {
			r.With(app.requireAuthenicatedUser).Group(func(r chi.Router) {
				r.Use(app.CheckPermissions(RequireRoles(store.VendorRole, store.AdminRole)))
				r.Get("/", app.getOptionTypes)
				r.Get("/{id}", app.getOptionTypeByID)
			})
		})

		r.Route("/products", func(r chi.Router) {
			r.Get("/", app.getProducts)

			r.Route("/whitelists", func(r chi.Router) {
				r.Use(app.requireAuthenicatedUser)
				r.With(app.CheckPermissions(RequireRoles(store.UserRole))).Group(func(r chi.Router) {
					r.Post("/", app.addProductToWhitelist)
					r.Delete("/{itemID}", app.removeProductFromWhitelist)
					r.Get("/", app.getGroupVendorWishlisttem)
					r.Get("/vendor", app.getVendorWishlistItem)
				})
			})

			r.Route("/{productID}", func(r chi.Router) {

				r.Get("/", app.getProduct)
				r.Route("/reviews", func(r chi.Router) {
					r.Get("/", app.getProductReviews)
					r.Get("/analytics", app.getReviewRatingAnalytics)

					r.With(app.requireAuthenicatedUser).Group(func(r chi.Router) {
						r.With(app.CheckPermissions(RequireRoles(store.UserRole))).Post("/", app.createReview)
					})
				})
			})

			r.Group(func(r chi.Router) {
				r.Use(app.requireAuthenicatedUser)

				r.With(app.CheckPermissions(RequireRoles(store.VendorRole))).Group(func(r chi.Router) {
					r.Post("/", app.createProduct)
					r.Patch("/{productID}/publish", app.publishProduct)
					r.Patch("/{productID}/unpublish", app.unPublishProduct)
				})

				r.With(app.CheckPermissions(RequireLevels(store.AdminLevelManager))).Group(func(r chi.Router) {
					r.Patch("/{productID}/approve", app.approveProduct)
					r.Patch("/{productID}/reject", app.rejectProduct)
				})
			})

		})

		r.Route("/admins", func(r chi.Router) {
			r.Use(app.requireAuthenicatedUser)
			r.Use(app.CheckPermissions(RequireRoles(store.AdminRole)))

			r.Route("/audit-logs", func(r chi.Router) {
				r.Get("/", app.getAuditLogs)
				r.Get("/{logID}", app.getAuditLogByID)
			})

			r.Route("/members", func(r chi.Router) {
				r.With(app.CheckPermissions(MinimumAdminLevel(store.AdminLevelSuper))).Post("/", app.createAdmin)
				r.Get("/", app.getAdminUsers)
				r.With(app.CheckPermissions(MinimumAdminLevel(store.AdminLevelManager))).Route("/{memberID}", func(r chi.Router) {

					r.Patch("/role", app.changeAdminRole)
					r.Patch("/enable", app.enableAdminAccount)
					r.Patch("/disable", app.disableAdminAccount)
				})
			})

			r.Route("/products", func(r chi.Router) {
				r.Route("/{productID}", func(r chi.Router) {
					r.Route("/reviews", func(r chi.Router) {
						r.Delete("/{reviewID}", app.removeReview)
					})
				})
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

			r.Route("/option-types", func(r chi.Router) {
				r.With(app.CheckPermissions(MinimumAdminLevel(store.AdminLevelManager))).Group(func(r chi.Router) {
					r.Post("/", app.createOptionType)
					r.Post("/{id}/values", app.addOptionValues)
					r.Put("/{id}", app.updateOptionType)
					r.Put("/values/{id}", app.updateOptionValue)
					r.Delete("/values/{id}", app.deleteOptionValue)
				})
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
			return
		}

		shutdownError <- nil

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
