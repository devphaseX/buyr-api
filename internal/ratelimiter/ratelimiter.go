package ratelimiter

import (
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/middleware/stdlib"
)

func NewRateLimit(store limiter.Store, rate limiter.Rate, keyGetter stdlib.KeyGetter) *stdlib.Middleware {
	limiter := limiter.New(store, rate)

	middleware := stdlib.NewMiddleware(limiter, stdlib.WithKeyGetter(keyGetter))
	return middleware
}
