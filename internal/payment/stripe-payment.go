package payment

import (
	"fmt"

	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
)

type StripePayment struct {
	successURL string
	cancelURL  string
}

func NewStripePayment(successURL, cancelURL string) *StripePayment {
	return &StripePayment{
		successURL: successURL,
		cancelURL:  cancelURL,
	}
}

func (s *StripePayment) InitiatePayment(order *store.Order, cartItems []*store.OrderItem) (string, error) {
	lineItems := []*stripe.CheckoutSessionLineItemParams{}

	for _, item := range cartItems {
		lineItems = append(lineItems, &stripe.CheckoutSessionLineItemParams{
			PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
				Currency: stripe.String("usd"),
				ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
					Name: stripe.String(item.Product.Name),
				},
				UnitAmount: stripe.Int64(int64(item.Price * 100)), // Convert to cents
			},
			Quantity: stripe.Int64(int64(item.Quantity)),
		})
	}

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{
			"card",
		}),
		LineItems:  lineItems,
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: stripe.String(s.successURL + "?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(s.cancelURL),
		Metadata: map[string]string{
			"order_id": order.ID,
		},
	}

	session, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("failed to create Stripe Checkout Session: %w", err)
	}

	return session.URL, nil
}
