package ratelimiter

// import (
// 	"context"
// 	"fmt"
// 	"net/http"
// 	"strconv"
// 	"time"

// 	"github.com/ulule/limiter/v3"
// 	"github.com/ulule/limiter/v3/drivers/store/redis"
// )

// type RateLimitStrategy string

// const (
// 	GlobalStrategy        RateLimitStrategy = "global"
// 	AuthenticatedStrategy RateLimitStrategy = "authenticated"
// 	AnonymousStrategy     RateLimitStrategy = "anonymous"
// )

// type RateLimitRule struct {
// 	Strategy RateLimitStrategy
// 	Limit    int64
// 	Period   time.Duration

// 	KeyFunc   func(r *http.Request) string
// 	Condition func(*http.Request) bool
// }

// type EndpointConfig struct {
// 	Path   string
// 	Method string
// 	Rules  []RateLimitRule
// }

// type RuleSelector interface {
// 	SelectRules(r *http.Request, rules []RateLimitRule) []RateLimitRule
// }

// // DefaultRuleSelector provides default rule selection logic
// type DefaultRuleSelector struct{}

// func (s *DefaultRuleSelector) SelectRules(r *http.Request, rules []RateLimitRule) []RateLimitRule {
// 	var selectedRules []RateLimitRule
// 	for _, rule := range rules {
// 		if rule.Condition == nil || rule.Condition(r) {
// 			selectedRules = append(selectedRules, rule)
// 		}
// 	}
// 	return selectedRules
// }

// type RateLimiter interface {
// 	Allow(ctx context.Context, key string) (bool, *limiter.Context, error)
// }

// type RateLimiterService struct {
// 	client       redis.Client
// 	ruleSelector RuleSelector
// 	configs      map[string]EndpointConfig
// 	limiters     map[string]map[RateLimitStrategy]RateLimiter
// }

// type ServiceOption func(*RateLimiterService)

// func WithRuleSelector(selector RuleSelector) ServiceOption {
// 	return func(s *RateLimiterService) {
// 		s.ruleSelector = selector
// 	}
// }

// func IsAuthenticated(r *http.Request) bool {
// 	return r.Header.Get("X-User-ID") != ""
// }

// func IsAdmin(r *http.Request) bool {
// 	return r.Header.Get("X-Role") == "admin"
// }

// func HasAPIKey(r *http.Request) bool {
// 	return r.Header.Get("X-API-Key") != ""
// }

// func NewRateLimiterService(client redis.Client, opts ...ServiceOption) (*RateLimiterService, error) {
// 	r := &RateLimiterService{
// 		client:       client,
// 		ruleSelector: &DefaultRuleSelector{},
// 		configs:      make(map[string]EndpointConfig),
// 		limiters:     make(map[string]map[RateLimitStrategy]RateLimiter),
// 	}

// 	for _, opt := range opts {
// 		opt(r)
// 	}

// 	return r, nil
// }

// func (s *RateLimiterService) AddEndpoint(config EndpointConfig) error {
// 	// Use a unique key for the endpoint and method combination
// 	key := fmt.Sprintf("%s:%s", config.Path, config.Method)
// 	s.configs[key] = config

// 	endpointLimiters := make(map[RateLimitStrategy]RateLimiter)

// 	for _, rule := range config.Rules {
// 		prefix := fmt.Sprintf("%s:%s:%s:", config.Path, config.Method, rule.Strategy)
// 		storeClone, err := redis.NewStoreWithOptions(s.client, limiter.StoreOptions{
// 			Prefix: prefix,
// 		})

// 		if err != nil {
// 			return err
// 		}

// 		rate := limiter.Rate{
// 			Limit:  rule.Limit,
// 			Period: rule.Period,
// 		}

// 		endpointLimiters[rule.Strategy] = &rateLimiterImpl{
// 			limiter: limiter.New(storeClone, rate),
// 		}
// 	}

// 	s.limiters[key] = endpointLimiters
// 	return nil
// }

// func (s *RateLimiterService) withRules(prefix string, rules ...RateLimitRule) ([]RateLimiter, error) {
// 	limiters := make([]RateLimiter, len(rules))

// 	for _, rule := range rules {
// 		storeClone, err := redis.NewStoreWithOptions(s.client, limiter.StoreOptions{
// 			Prefix: prefix,
// 		})

// 		if err != nil {
// 			return nil, err
// 		}

// 		rate := limiter.Rate{
// 			Limit:  rule.Limit,
// 			Period: rule.Period,
// 		}

// 		limiters = append(limiters, &rateLimiterImpl{
// 			limiter: limiter.New(storeClone, rate),
// 		})

// 	}
// 	return limiters, nil
// }

// func (s *RateLimiterService) Middleware() func(http.Handler) http.Handler {
// 	return func(next http.Handler) http.Handler {
// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			// Find the endpoint configuration for the requested path and method
// 			var config EndpointConfig
// 			for _, cfg := range s.configs {
// 				if cfg.Path == r.URL.Path && cfg.Method == r.Method {
// 					config = cfg
// 					break
// 				}
// 			}

// 			// If no configuration is found, proceed to the next handler
// 			if config.Path == "" {
// 				next.ServeHTTP(w, r)
// 				return
// 			}

// 			// Select applicable rules based on request context
// 			applicableRules := s.ruleSelector.SelectRules(r, config.Rules)
// 			limiters := s.limiters[config.Path]

// 			for _, rule := range applicableRules {
// 				limiter := limiters[rule.Strategy]

// 				var key = rule.KeyFunc(r)
// 				if key == "" {
// 					continue
// 				}

// 				allowed, context, err := limiter.Allow(r.Context(), key)
// 				if err != nil {
// 					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
// 					return
// 				}
// 				if !allowed {
// 					// Set rate limit headers
// 					w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(context.Limit, 10))
// 					w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(context.Remaining, 10))
// 					w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(context.Reset, 10))
// 					w.Header().Set("X-RateLimit-Strategy", string(rule.Strategy))
// 					http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
// 					return
// 				}

// 				// Set rate limit headers for allowed requests
// 				w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(context.Limit, 10))
// 				w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(context.Remaining, 10))
// 				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(context.Reset, 10))
// 				w.Header().Set("X-RateLimit-Strategy", string(rule.Strategy))
// 			}

// 			next.ServeHTTP(w, r)
// 		})
// 	}
// }

// type rateLimiterImpl struct {
// 	limiter *limiter.Limiter
// }

// func (r *rateLimiterImpl) Allow(ctx context.Context, key string) (bool, *limiter.Context, error) {
// 	context, err := r.limiter.Get(ctx, key)
// 	if err != nil {
// 		return false, nil, err
// 	}
// 	return !context.Reached, &context, nil
// }
