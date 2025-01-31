package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/store"
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
	productsCount := make(map[string]int)
	productsPrice := make(map[string]float64)

	for _, item := range cartItems {
		cartItemIds = append(cartItemIds, item.ID)
		productsCount[item.ProductID] = item.Quantity
	}

	products, err := app.store.Products.GetProductsByIDS(r.Context(), cartItemIds)

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

	_ = orderID

	// // Fetch the order from the database.
	order, err := app.store.Orders.GetByID(r.Context(), user.ID, orderID)
	if err != nil {
		if errors.Is(err, store.ErrRecordNotFound) {
			app.notFoundResponse(w, r, "order not found")
		} else {
			app.serverErrorResponse(w, r, fmt.Errorf("failed to fetch order: %w", err))
		}
		return
	}

	// Ensure the order is in a "pending" state.
	if order.Status != "pending" {
		app.badRequestResponse(w, r, fmt.Errorf("order is already %s", order.Status))
		return
	}

}

func (app *application) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	// const MaxBodyBytes = int64(65536)
	// r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	// payload, err := io.ReadAll(r.Body)
	// if err != nil {
	// 	app.serverErrorResponse(w, r, fmt.Errorf("failed to read webhook payload: %w", err))
	// 	return
	// }

	// event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), app.config.stripeWebhookSecret)
	// if err != nil {
	// 	app.serverErrorResponse(w, r, fmt.Errorf("failed to verify webhook signature: %w", err))
	// 	return
	// }

	// switch event.Type {
	// case "checkout.session.completed":
	// 	var session stripe.CheckoutSession
	// 	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
	// 		app.serverErrorResponse(w, r, fmt.Errorf("failed to parse checkout session: %w", err))
	// 		return
	// 	}

	// 	// Extract the order ID from the metadata.
	// 	orderID := session.Metadata["order_id"]

	// 	// Enqueue the payment processing task.
	// 	payload, err := json.Marshal(ProcessPaymentPayload{
	// 		OrderID: orderID,
	// 		Amount:  session.AmountTotal / 100, // Convert from cents to dollars
	// 	})
	// 	if err != nil {
	// 		app.serverErrorResponse(w, r, fmt.Errorf("failed to marshal payload: %w", err))
	// 		return
	// 	}

	// 	task := asynq.NewTask("process_payment", payload)
	// 	if _, err := app.asynqClient.Enqueue(task); err != nil {
	// 		app.serverErrorResponse(w, r, fmt.Errorf("failed to enqueue payment task: %w", err))
	// 		return
	// 	}

	// default:
	// 	app.logger.Info("unhandled Stripe event type", "type", event.Type)
	// }

	// w.WriteHeader(http.StatusOK)
}
