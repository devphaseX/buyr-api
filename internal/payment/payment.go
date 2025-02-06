package payment

import (
	"errors"

	"github.com/devphaseX/buyr-api.git/internal/store"
)

type Payment interface {
	InitiatePayment(order *store.Order, cartItems []*store.OrderItem) (string, error)
}

type Config struct {
	Stripe StripeConfig
}

type StripeConfig struct {
	SuccessURL string
	CancelURL  string
}

func NewPayment(paymentMethod string, cfg *Config) (Payment, error) {
	switch paymentMethod {
	case "stripe":
		return NewStripePayment(cfg.Stripe.SuccessURL, cfg.Stripe.CancelURL), nil
	// case "paypal":
	//      return &PayPalPayment{ /* ... */ }, nil
	default:
		return nil, errors.New("invalid payment method")
	}
}
