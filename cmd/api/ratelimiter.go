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
	app.rateLimitService.AddEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/products",
		Method: http.MethodGet,
		Rules: []ratelimiter.RateLimitRule{{
			Strategy:  ratelimiter.AnonymousStrategy,
			Limit:     5,
			Period:    time.Minute,
			KeyFunc:   ipBaseRateLimiterGetter,
			Condition: ratelimiter.IsNotAuthenticated,
		},
			{
				Strategy:  ratelimiter.AuthenticatedStrategy,
				Limit:     20,
				Period:    time.Minute,
				KeyFunc:   userBaseRateLimiterGetter,
				Condition: ratelimiter.IsAuthenticated,
			},
		},
	})

	app.rateLimitService.AddEndpoint(ratelimiter.EndpointConfig{
		Path:   "/v1/categories",
		Method: http.MethodGet,
		Rules: []ratelimiter.RateLimitRule{{
			Strategy: ratelimiter.AnonymousStrategy,
			Limit:    5,
			Period:   time.Minute,
			KeyFunc:  ipBaseRateLimiterGetter,
		}},
	})
}
