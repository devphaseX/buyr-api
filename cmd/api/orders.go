package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/devphaseX/buyr-api.git/worker"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/webhook"
)

type createOrderRequest struct {
	CartItems []string `json:"cart_items" validate:"required"`
	PromoCode string   `json:"promo_code"`
}

func (app *application) createOrder(w http.ResponseWriter, r *http.Request) {
	var (
		form createOrderRequest
		user = getUserFromCtx(r)
	)

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	cart, err := app.store.Carts.GetCartByUserID(user.ID)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	cartItems, err := app.store.CartItems.GetItemsByIDS(r.Context(), cart.ID, form.CartItems)

	if len(cartItems) != len(form.CartItems) {
		app.errorResponse(w, http.StatusUnprocessableEntity, "one or more cart items do not exist")
		return
	}

	cartItemIds := make([]string, len(cartItems))
	cartItemProductIds := make([]string, len(cartItems))
	productsCount := make(map[string]int)
	productsPrice := make(map[string]float64)

	for _, item := range cartItems {
		cartItemIds = append(cartItemIds, item.ID)
		cartItemProductIds = append(cartItemProductIds, item.ProductID)
		productsCount[item.ProductID] = item.Quantity
	}

	products, err := app.store.Products.GetProductsByIDS(r.Context(), cartItemProductIds)

	if len(cartItems) != len(products) {
		app.errorResponse(w, http.StatusUnprocessableEntity, "one or more cart items do not exist or out of stock")
		return
	}

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	var totalPrice float64
	// Check if all products are in stock and have sufficient quantity.
	for _, item := range products {
		if item.StockQuantity <= productsCount[item.ID] {
			app.badRequestResponse(w, r, fmt.Errorf("product %s is out of stock", item.Name))
			return
		}

		totalPrice += float64(productsCount[item.ID]) * (item.Price - item.Discount)
		productsPrice[item.ID] = item.Price - item.Discount

	}

	for _, item := range cartItems {
		item.Price = productsPrice[item.ProductID]
	}

	order := &store.Order{
		UserID:      user.ID,
		TotalAmount: totalPrice,
		PromoCode:   form.PromoCode,
		Status:      store.PendingOrderStatus,
	}

	err = app.store.Orders.Create(r.Context(), app.store.Products, order, cartItems)

	if err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to create order: %w", err))
		return
	}

	response := envelope{
		"message": "order created successfully",
		"data": map[string]interface{}{
			"order_id": order.ID,
			"payment_options": []string{
				"stripe",
				"paypal",
			},
		},
	}

	app.successResponse(w, http.StatusCreated, response)
}

/*
	// Apply the promo code (if provided) to calculate discounts.
		if form.PromoCode != "" {
			promo, err := app.store.Promos.GetPromoByCode(r.Context(), form.PromoCode)
			if err != nil {
				if errors.Is(err, store.ErrRecordNotFound) {
					app.badRequestResponse(w, r, errors.New("invalid promo code"))
				} else {
					app.serverErrorResponse(w, r, fmt.Errorf("failed to fetch promo code: %w", err))
				}
				return
			}

			// Validate the promo code.
			if time.Now().After(promo.ExpiresAt) {
				app.badRequestResponse(w, r, errors.New("promo code has expired"))
				return
			}

			// Apply the discount to the total price.
			totalPrice -= totalPrice * (promo.DiscountPercent / 100)
		}

		// Deduct the stock quantity for each product in the cart.
		for _, item := range cartItems {
			_, err := tx.ExecContext(r.Context(), `
				UPDATE products
				SET stock_quantity = stock_quantity - 1
				WHERE id = $1 AND stock_quantity > 0
			`, item.ID)
			if err != nil {
				app.serverErrorResponse(w, r, fmt.Errorf("failed to update stock for product %s: %w", item.Name, err))
				return
			}
		}
*/

type initialPaymentRequest struct {
	PaymentMethod string `json:"payment_method" validate:"required,oneof=stripe paypal"`
}

func (app *application) initiatePayment(w http.ResponseWriter, r *http.Request) {
	var (
		form initialPaymentRequest
		user = getUserFromCtx(r)
	)

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Extract the order ID from the URL.
	orderID := app.readStringID(r, "orderID")

	// // Fetch the order from the database.
	order, err := app.store.Orders.GetUserOrderByID(r.Context(), user.ID, orderID)

	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			app.notFoundResponse(w, r, "order not found")
		} else {
			app.serverErrorResponse(w, r, fmt.Errorf("failed to fetch order: %w", err))
		}
		return
	}

	if order.Status != "pending" {
		app.badRequestResponse(w, r, fmt.Errorf("order is already %s", order.Status))
		return
	}

	cartItems, err := app.store.OrderItems.GetItemsByOrderID(r.Context(), order.ID)

	if err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to fetch order items: %w", err))
		return
	}
	switch form.PaymentMethod {
	case "stripe":

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
			SuccessURL: stripe.String(app.cfg.stripe.successURL + "?session_id={CHECKOUT_SESSION_ID}"),
			CancelURL:  stripe.String(app.cfg.stripe.cancelURL),
			Metadata: map[string]string{
				"order_id": orderID,
			},
		}

		// Create the Stripe Checkout Session.
		session, err := session.New(params)
		if err != nil {
			app.serverErrorResponse(w, r, fmt.Errorf("failed to create Stripe Checkout Session: %w", err))
			return
		}

		// // Return the Stripe Checkout Session URL to the client.
		response := envelope{
			"message": "payment initiated successfully",
			"data": map[string]interface{}{
				"payment_url": session.URL,
			},
		}
		app.successResponse(w, http.StatusOK, response)

	default:
		app.badRequestResponse(w, r, errors.New("invalid payment method"))
	}

}

func (app *application) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	const MaxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to read webhook payload: %w", err))
		return
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), app.cfg.stripe.webhookSecret)
	if err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to verify webhook signature: %w", err))
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			app.serverErrorResponse(w, r, fmt.Errorf("failed to parse checkout session: %w", err))
			return
		}

		// Extract the order ID from the metadata.
		orderID := session.Metadata["order_id"]
		paymentID := event.Data.Object["payment_intent"].(string)
		PaymentStatus := event.Data.Object["payment_status"].(string)
		var status = store.FailedPaymentStatus

		if PaymentStatus == "paid" {
			status = store.CompletedPaymentStatus
		}

		err := app.taskDistributor.DistributeTaskProcessOrderPayment(r.Context(), &worker.ProcessPaymentPayload{
			OrderID:       orderID,
			Amount:        float64(session.AmountTotal / 100), // Convert from cents to dollars
			TransactionID: paymentID,
			Status:        status,
		})

		if err != nil {
			app.serverErrorResponse(w, r, fmt.Errorf("failed to send to payment process queue: %w", err))
		}

		return

	default:
		app.logger.Info("unhandled Stripe event type", "type", event.Type)
	}

	w.WriteHeader(http.StatusOK)
}
