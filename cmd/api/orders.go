package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/payment"
	"github.com/devphaseX/buyr-api.git/internal/store"
	"github.com/devphaseX/buyr-api.git/worker"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
)

type createOrderRequest struct {
	CartItems         []string `json:"cart_items" validate:"required"`
	PromoCode         string   `json:"promo_code"`
	ShippingAddressID string   `json:"shipping_address_id" validate:"required"`
}

func (app *application) handleCheckout(w http.ResponseWriter, r *http.Request) {
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

	var (
		productsCount      = make(map[string]int)
		productsPrice      = make(map[string]float64)
		cartItemProductIds = make([]string, len(cartItems))
	)

	for _, item := range cartItems {
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

	address, err := app.store.Address.GetByID(r.Context(), form.ShippingAddressID)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "shipping address not found")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if address.UserID != user.ID {
		app.notFoundResponse(w, r, "shipping address not found")
		return
	}

	if address.AddressType == store.ShippingAddressType {
		app.forbiddenResponse(w, r, "invalid address type for shipping")
		return
	}

	var totalPrice float64
	// Check if all products are in stock and have sufficient quantity.
	for _, item := range products {
		if item.StockQuantity <= productsCount[item.ID] {
			app.badRequestResponse(w, r, fmt.Errorf("insufficient stock for product '%s'. Available quantity: %d", item.Name, item.StockQuantity))
			return
		}

		totalPrice += float64(productsCount[item.ID]) * (item.Price - item.Discount)
		productsPrice[item.ID] = item.Price - item.Discount

	}

	var promo *store.Promo
	if form.PromoCode != "" {
		promo, totalPrice, err = app.store.Promos.ValidatePromoCode(r.Context(), form.PromoCode, user.ID, totalPrice)
		if err != nil {
			switch {
			case errors.Is(err, store.ErrPromoNotFound):
				app.errorResponse(w, http.StatusNotFound, "promo code not found")
			case errors.Is(err, store.ErrPromoExpired):
				app.errorResponse(w, http.StatusBadRequest, "promo code has expired")
			case errors.Is(err, store.ErrPromoUsageLimitReached):
				app.errorResponse(w, http.StatusBadRequest, "promo code has reached its usage limit")
			case errors.Is(err, store.ErrMinPurchaseNotMet):
				app.errorResponse(w, http.StatusBadRequest, err.Error())
			case errors.Is(err, store.ErrUserNotAllowed):
				app.errorResponse(w, http.StatusForbidden, "promo code is not valid for this user")
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}
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

	if promo != nil {
		err = app.store.Promos.IncrementUsage(r.Context(), promo.ID)
		if err != nil {
			app.serverErrorResponse(w, r, fmt.Errorf("failed to increment promo usage: %w", err))
			return
		}
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

	orderItems, err := app.store.OrderItems.GetItemsByOrderID(r.Context(), order.ID)

	if err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to fetch order items: %w", err))
		return
	}

	payment, err := payment.NewPayment(form.PaymentMethod, &payment.Config{
		Stripe: payment.StripeConfig{
			SuccessURL: app.cfg.stripe.successURL,
			CancelURL:  app.cfg.stripe.cancelURL,
		},
	})

	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	paymentURL, err := payment.InitiatePayment(order, orderItems)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"message": "payment initiated successfully",
		"data": envelope{
			"payment_url": paymentURL,
		},
	}
	app.successResponse(w, http.StatusOK, response)

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

func (app *application) getUserViewOrderLists(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	fq := store.PaginateQueryFilter{
		Page:         1,
		PageSize:     20,
		Sort:         "created_at",
		SortSafelist: []string{"created_at", "-created_at"},
	}

	if err := fq.Parse(r); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	orders, metadata, err := app.store.Orders.GetOrdersForUser(r.Context(), user.ID, fq)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"orders":   orders,
		"metadata": metadata,
	}

	app.successResponse(w, http.StatusOK, response)
}

func (app *application) getOrderForUserByID(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	orderID := app.readStringID(r, "orderID")

	order, err := app.store.Orders.GetOrderForUserByID(r.Context(), user.ID, orderID)

	if err != nil {
		switch {

		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "order not found")

		default:
			app.serverErrorResponse(w, r, err)
		}

		return
	}

	response := envelope{
		"order": order,
	}

	app.successResponse(w, http.StatusOK, response)
}
