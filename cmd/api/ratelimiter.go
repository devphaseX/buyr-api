package main

import (
	"net/http"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/ratelimiter"
)

func userBaseRateLimiterGetter(r *http.Request) string {
	user := getUserFromCtx(r)
	return user.ID
}

func ipBaseRateLimiterGetter(r *http.Request) string {
	return r.RemoteAddr
}

func (app *application) setRateLimit() {
	var err error

	addEndpoint := func(config ratelimiter.EndpointConfig) {
		err = app.rateLimitService.AddEndpoint(config)
		if err != nil {
			app.logger.Fatalf("Error adding endpoint config for %s %s: %v", config.Method, config.Path, err)
		}
	}

	// ================== Public Endpoints ==================

	// GET /v1/csrf-token
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/csrf-token",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AnonymousStrategy,
			Limit:     60,
			Period:    time.Minute,
			KeyFunc:   ipBaseRateLimiterGetter,
			Condition: ratelimiter.IsNotAuthenticated,
		}},
	})

	// GET /v1/categories
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/categories",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AnonymousStrategy,
			Limit:     60,
			Period:    time.Minute,
			KeyFunc:   ipBaseRateLimiterGetter,
			Condition: ratelimiter.IsNotAuthenticated,
		}},
	})

	// ================== Authentication Endpoints ==================

	// POST /v1/auth/register
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/auth/register",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AnonymousStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   ipBaseRateLimiterGetter,
			Condition: ratelimiter.IsNotAuthenticated,
		}},
	})

	// POST /v1/auth/sign-in
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/auth/sign-in",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AnonymousStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   ipBaseRateLimiterGetter,
			Condition: ratelimiter.IsNotAuthenticated,
		}},
	})

	// POST /v1/auth/sign-in/2fa
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/auth/sign-in/2fa",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AnonymousStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   ipBaseRateLimiterGetter,
			Condition: ratelimiter.IsNotAuthenticated,
		}},
	})

	// POST /v1/auth/sign-in/recovery-code
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/auth/sign-in/recovery-code",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AnonymousStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   ipBaseRateLimiterGetter,
			Condition: ratelimiter.IsNotAuthenticated,
		}},
	})

	// POST /v1/auth/refresh
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/auth/refresh",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     20,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// POST /v1/auth/forget-password
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/auth/forget-password",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AnonymousStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   ipBaseRateLimiterGetter,
			Condition: ratelimiter.IsNotAuthenticated,
		}},
	})

	// POST /v1/auth/reset-password/verify-email
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/auth/reset-password/verify-email",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AnonymousStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   ipBaseRateLimiterGetter,
			Condition: ratelimiter.IsNotAuthenticated,
		}},
	})

	// POST /v1/auth/reset-password/2fa
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/auth/reset-password/2fa",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AnonymousStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   ipBaseRateLimiterGetter,
			Condition: ratelimiter.IsNotAuthenticated,
		}},
	})

	// POST /v1/auth/reset-password/recovery-code
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/auth/reset-password/recovery-code",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AnonymousStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   ipBaseRateLimiterGetter,
			Condition: ratelimiter.IsNotAuthenticated,
		}},
	})

	// POST /v1/auth/reset-password/change
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/auth/reset-password/change",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AnonymousStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   ipBaseRateLimiterGetter,
			Condition: ratelimiter.IsNotAuthenticated,
		}},
	})

	// POST /v1/auth/google
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/auth/google",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AnonymousStrategy,
			Limit:     10,
			Period:    time.Minute,
			KeyFunc:   ipBaseRateLimiterGetter,
			Condition: ratelimiter.IsNotAuthenticated,
		}},
	})

	// POST /v1/auth/google/callback
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/auth/google/callback",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AnonymousStrategy,
			Limit:     10,
			Period:    time.Minute,
			KeyFunc:   ipBaseRateLimiterGetter,
			Condition: ratelimiter.IsNotAuthenticated,
		}},
	})

	// ================== User Endpoints ==================

	// PATCH /v1/users/activate-account
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/users/activate-account",
		Method: "PATCH",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// PATCH /v1/users/email/verify-email
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/users/email/verify-email",
		Method: "PATCH",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// GET /v1/users (requires authentication)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/users",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     60,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// GET /v1/users/current (requires authentication)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/users/current",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     60,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// PATCH /v1/users/change-password (requires authentication)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/users/change-password",
		Method: "PATCH",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     10,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// PATCH /v1/users/change-password/2fa (requires authentication)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/users/change-password/2fa",
		Method: "PATCH",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// POST /v1/users/email/initiate-change (requires authentication)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/users/email/initiate-change",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// POST /v1/users/email/verify-2fa (requires authentication)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/users/email/verify-2fa",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// ================== File Endpoints ==================

	// POST /v1/files/image
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/files/image",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// ================== MFA Endpoints ==================

	// GET /v1/mfa/setup (requires authentication and admin/vendor roles)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/mfa/setup",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     10,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// POST /v1/mfa/verify (requires authentication and admin/vendor roles)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/mfa/verify",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     10,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// POST /v1/mfa/recovery-codes (requires authentication and admin/vendor roles)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/mfa/recovery-codes",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     10,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// PATCH /v1/mfa/recovery-codes/reset (requires authentication and admin/vendor roles)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/mfa/recovery-codes/reset",
		Method: "PATCH",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// ================== Cart Endpoints ==================

	// GET /v1/carts (requires authentication and user role)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/carts",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     30,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// GET /v1/carts/items (requires authentication and user role)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/carts/items",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     30,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// GET /v1/carts/items/vendor (requires authentication and user role)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/carts/items/vendor",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     30,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// ================== Cart Endpoints (continued) ==================

	// POST /v1/carts/{orderID}/pay (requires authentication and user role)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/carts/{orderID}/pay",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// POST /v1/carts/items (requires authentication and user role)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/carts/items",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     10,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// GET /v1/carts/items/{cardItemID} (requires authentication and user role)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/carts/items/{cardItemID}",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     30,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// DELETE /v1/carts/items/{itemID} (requires authentication and user role)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/carts/items/{itemID}",
		Method: "DELETE",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     10,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// PATCH /v1/carts/items/{itemID} (requires authentication and user role)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/carts/items/{itemID}",
		Method: "PATCH",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     10,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// ================== Order Endpoints ==================

	// POST /v1/orders/webhook/stripe
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/orders/webhook/stripe",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AnonymousStrategy,
			Limit:     10,
			Period:    time.Minute,
			KeyFunc:   ipBaseRateLimiterGetter,
			Condition: ratelimiter.IsNotAuthenticated,
		}},
	})

	// GET /v1/orders (requires authentication and user role)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/orders",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     30,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// GET /v1/orders/{orderID} (requires authentication and user role)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/orders/{orderID}",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     30,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// ================== Address Endpoints ==================

	// GET /v1/addresses (requires authentication)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/addresses",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     30,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// POST /v1/addresses (requires authentication)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/addresses",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     10,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// GET /v1/addresses/{addressID} (requires authentication)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/addresses/{addressID}",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     30,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// GET /v1/addresses/default (requires authentication)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/addresses/default",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     30,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// ================== Option Type Endpoints ==================

	// GET /v1/option-types (requires authentication and vendor/admin roles)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/option-types",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     30,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// GET /v1/option-types/{id} (requires authentication and vendor/admin roles)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/option-types/{id}",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     30,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// ================== Product Endpoints ==================

	// GET /v1/products
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/products",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{
			{
				Strategy:  ratelimiter.AnonymousStrategy,
				Limit:     60,
				Period:    time.Minute,
				KeyFunc:   ipBaseRateLimiterGetter,
				Condition: ratelimiter.IsNotAuthenticated,
			},
			{
				Strategy:  ratelimiter.AuthenticatedStrategy,
				Limit:     120,
				Period:    time.Minute,
				KeyFunc:   userBaseRateLimiterGetter,
				Condition: ratelimiter.IsAuthenticated,
			},
		},
	})

	// POST /v1/products/whitelists (requires authentication and user role)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/products/whitelists",
		Method: "POST",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     10,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// DELETE /v1/products/whitelists/{itemID} (requires authentication and user role)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/products/whitelists/{itemID}",
		Method: "DELETE",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     10,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// GET /v1/products/whitelists (requires authentication and user role)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/products/whitelists",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     30,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// GET /v1/products/whitelists/vendor (requires authentication and user role)
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/products/whitelists/vendor",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AuthenticatedStrategy,
			Limit:     30,
			Period:    time.Minute,
			KeyFunc:   userBaseRateLimiterGetter,
			Condition: ratelimiter.IsAuthenticated,
		}},
	})

	// GET /v1/products/{productID}
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/products/{productID}",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AnonymousStrategy,
			Limit:     60,
			Period:    time.Minute,
			KeyFunc:   ipBaseRateLimiterGetter,
			Condition: ratelimiter.IsNotAuthenticated,
		}},
	})

	// GET /v1/products/{productID}/reviews
	addEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/products/{productID}/reviews",
		Method: "GET",
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AnonymousStrategy,
			Limit:     60,
			Period:    time.Minute,
			KeyFunc:   ipBaseRateLimiterGetter,
			Condition: ratelimiter.IsNotAuthenticated,
		}},
	})
}
