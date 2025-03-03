package ratelimiter

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/redis"
)

type RateLimitStrategy string

const (
	GlobalStrategy        RateLimitStrategy = "global"
	AuthenticatedStrategy RateLimitStrategy = "authenticated"
	AnonymousStrategy     RateLimitStrategy = "anonymous"
)

type RateLimitRule struct {
	Strategy RateLimitStrategy
	Limit    int64
	Period   time.Duration

	KeyFunc   func(r *http.Request) string
	Condition func(*http.Request) bool
}

type EndpointConfig struct {
	Path   string
	Method string
	Rules  []RateLimitRule
}

type RuleSelector interface {
	SelectRules(r *http.Request, rules []RateLimitRule) []RateLimitRule
}

// DefaultRuleSelector provides default rule selection logic
type DefaultRuleSelector struct{}

func (s *DefaultRuleSelector) SelectRules(r *http.Request, rules []RateLimitRule) []RateLimitRule {
	var selectedRules []RateLimitRule
	for _, rule := range rules {
		if rule.Condition == nil || rule.Condition(r) {
			selectedRules = append(selectedRules, rule)
		}
	}
	return selectedRules
}

// PathMatcher defines an interface for path matching.
type PathMatcher interface {
	Match(registeredPath, requestPath string) bool
}

// PatternPathMatcher implements PathMatcher using a pattern which supports dynamic matching.
type PatternPathMatcher struct{}

func (p *PatternPathMatcher) Match(registeredPath, requestPath string) bool {
	// Escape the registered path for use in the regex.
	regexString := "^" + regexp.QuoteMeta(registeredPath) + "$"

	fmt.Println("before Regex:", regexString)

	// Replace the dynamic parameters with regex groups.
	regexString = replaceDynamicParams(regexString)

	regex, err := regexp.Compile(regexString)
	if err != nil {
		// Handle regex error.
		return false
	}

	fmt.Println("Regex:", regexString)

	return regex.MatchString(requestPath)
}

// replaceDynamicParams replaces dynamic parameters in the path with regex groups.
func replaceDynamicParams(path string) string {
	// This regex matches any {param} in the path.
	paramRegex := regexp.MustCompile(`\\\{[^\/]+\\\}`)
	// Replace {param} with ([^/]+) which matches any character except /.
	return paramRegex.ReplaceAllString(path, `([^/]+)`)
}

type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, *limiter.Context, error)
}

type LimitReachedHandler func(w http.ResponseWriter, r *http.Request)

type RateLimiterService struct {
	client         redis.Client
	ruleSelector   RuleSelector
	pathMatcher    PathMatcher
	OnLimitReached LimitReachedHandler
	configs        map[string]EndpointConfig
	limiters       map[string]map[RateLimitStrategy]RateLimiter
}

type ServiceOption func(*RateLimiterService)

func WithRuleSelector(selector RuleSelector) ServiceOption {
	return func(s *RateLimiterService) {
		s.ruleSelector = selector
	}
}

func WithLimitReachedHandler(handler LimitReachedHandler) ServiceOption {
	return func(s *RateLimiterService) {
		s.OnLimitReached = handler
	}
}

func IsAuthenticated(r *http.Request) bool {
	return r.Header.Get("X-User-ID") != ""
}
func IsNotAuthenticated(r *http.Request) bool {
	return !IsAuthenticated(r)
}

func IsAdmin(r *http.Request) bool {
	return r.Header.Get("X-Role") == "admin"
}

func HasAPIKey(r *http.Request) bool {
	return r.Header.Get("X-API-Key") != ""
}

func NewRateLimiterService(client redis.Client, opts ...ServiceOption) (*RateLimiterService, error) {
	r := &RateLimiterService{
		client:       client,
		ruleSelector: &DefaultRuleSelector{},
		pathMatcher:  &PatternPathMatcher{},
		configs:      make(map[string]EndpointConfig),
		limiters:     make(map[string]map[RateLimitStrategy]RateLimiter),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r, nil
}

func (s *RateLimiterService) createKey(path, method string) string {
	return fmt.Sprintf("%s:%s", path, method)
}

func (s *RateLimiterService) AddEndpoint(config EndpointConfig) error {
	// Use a unique key for the endpoint and method combination
	key := s.createKey(config.Path, config.Method)
	s.configs[key] = config

	endpointLimiters := make(map[RateLimitStrategy]RateLimiter)

	for _, rule := range config.Rules {

		prefix := fmt.Sprintf("%s:%s:%s:", config.Path, config.Method, rule.Strategy)
		storeClone, err := redis.NewStoreWithOptions(s.client, limiter.StoreOptions{
			Prefix: prefix,
		})

		if err != nil {
			return err
		}

		rate := limiter.Rate{
			Limit:  rule.Limit,
			Period: rule.Period,
		}

		limiter := limiter.New(storeClone, rate)

		endpointLimiters[rule.Strategy] = &rateLimiterImpl{
			limiter: limiter,
		}
	}

	s.limiters[key] = endpointLimiters
	return nil
}

func (s *RateLimiterService) withRules(prefix string, rules ...RateLimitRule) ([]RateLimiter, error) {
	limiters := make([]RateLimiter, len(rules))

	for _, rule := range rules {
		storeClone, err := redis.NewStoreWithOptions(s.client, limiter.StoreOptions{
			Prefix: prefix,
		})

		if err != nil {
			return nil, err
		}

		rate := limiter.Rate{
			Limit:  rule.Limit,
			Period: rule.Period,
		}

		limiters = append(limiters, &rateLimiterImpl{
			limiter: limiter.New(storeClone, rate),
		})

	}
	return limiters, nil
}

func (s *RateLimiterService) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Find the endpoint configuration for the requested path and method
			var config *EndpointConfig
			for _, cfg := range s.configs {
				if cfg.Method == r.Method && s.pathMatcher.Match(cfg.Path, r.URL.Path) {
					config = &cfg
					break
				}
			}

			// If no configuration is found, proceed to the next handler
			if config == nil || config.Path == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Select applicable rules based on request context
			applicableRules := s.ruleSelector.SelectRules(r, config.Rules)
			limiters := s.limiters[s.createKey(r.URL.Path, r.Method)]

			for _, rule := range applicableRules {
				limiter := limiters[rule.Strategy]

				var key = rule.KeyFunc(r)
				if key == "" {
					continue
				}

				allowed, context, err := limiter.Allow(r.Context(), key)
				if err != nil {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				if !allowed {
					// Set rate limit headers
					w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(context.Limit, 10))
					w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(context.Remaining, 10))
					w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(context.Reset, 10))
					w.Header().Set("X-RateLimit-Strategy", string(rule.Strategy))

					if s.OnLimitReached != nil {
						s.OnLimitReached(w, r)
					}
					return
				}

				// Set rate limit headers for allowed requests
				w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(context.Limit, 10))
				w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(context.Remaining, 10))
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(context.Reset, 10))
				w.Header().Set("X-RateLimit-Strategy", string(rule.Strategy))
			}

			next.ServeHTTP(w, r)
		})
	}
}

type rateLimiterImpl struct {
	limiter *limiter.Limiter
}

func (r *rateLimiterImpl) Allow(ctx context.Context, key string) (bool, *limiter.Context, error) {
	context, err := r.limiter.Get(ctx, key)
	if err != nil {
		return false, nil, err
	}
	return !context.Reached, &context, nil
}
