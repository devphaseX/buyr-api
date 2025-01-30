package main

import (
	"errors"
	"net/http"

	"github.com/devphaseX/buyr-api.git/internal/store"
)

func (app *application) getCurrentUserCart(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	cart, err := app.store.Carts.GetCartByUserID(user.ID)

	if !(err == nil || errors.Is(err, store.ErrRecordNotFound)) {
		app.serverErrorResponse(w, r, err)
		return
	}

	if cart == nil {
		cart, err = app.store.Carts.CreateCart(user.ID)
	}

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"cart": cart,
	}

	app.successResponse(w, http.StatusOK, response)
}

func (app *application) getGroupVendorCartItem(w http.ResponseWriter, r *http.Request) {

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

	user := getUserFromCtx(r)

	cart, err := app.store.Carts.GetCartByUserID(user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	cartItems, metadata, err := app.store.CartItems.GetGroupCartItems(r.Context(), cart.ID, 3, fq)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"vendor_cart_items": cartItems,
		"metadata":          metadata,
	}

	app.successResponse(w, http.StatusOK, response)
}

func (app *application) getVendorCartItem(w http.ResponseWriter, r *http.Request) {
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

	vendorID := r.URL.Query().Get("vendor_id")
	if vendorID == "" {
		app.badRequestResponse(w, r, errors.New("vendor_id missing in query"))
		return
	}

	vendor, err := app.store.Users.GetVendorByID(r.Context(), vendorID)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "vendor not found")

		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	user := getUserFromCtx(r)
	cart, err := app.store.Carts.GetCartByUserID(user.ID)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	cartItems, metadata, err := app.store.CartItems.GetCartItems(r.Context(), cart.ID, vendor.ID, fq)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"vendor_cart_items": cartItems,
		"metadata":          metadata,
	}

	app.successResponse(w, http.StatusOK, response)
}

func (app *application) getCartItemByID(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	cardItemID := app.readStringID(r, "cardItemID")

	cart, err := app.store.Carts.GetCartByUserID(user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	cartItem, err := app.store.CartItems.GetItemByID(r.Context(), cart.ID, cardItemID)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "cart item not found")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	response := envelope{
		"cart_item": cartItem,
	}

	app.successResponse(w, http.StatusOK, response)
}

type addCardItemRequest struct {
	ProductID string `json:"product_id" validate:"required"`
	Quantity  int    `json:"quantity" validate:"min=1"`
}

func (app *application) addCardItem(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)

	var (
		form addCardItemRequest
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

	product, err := app.store.Products.GetProductByID(r.Context(), form.ProductID)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "product not found")

		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if product.StockQuantity < form.Quantity {
		app.forbiddenResponse(w, r, "product not in stock")
		return
	}

	cartItem, err := app.store.CartItems.AddItem(r.Context(), cart.ID, product.ID, form.Quantity)

	if err != nil {
		switch {
		case errors.Is(err, store.ErrProductAlreadyCarted):
			app.conflictResponse(w, r, "item already in cart")

		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	response := envelope{
		"cart_item": cartItem,
	}

	app.successResponse(w, http.StatusCreated, response)
}

func (app *application) removeCartItem(w http.ResponseWriter, r *http.Request) {
	// Get the authenticated user from the context
	user := getUserFromCtx(r)
	itemID := app.readStringID(r, "itemID")

	// Get the user's cart
	cart, err := app.store.Carts.GetCartByUserID(user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Check if the cart item belongs to the user's cart
	cartItem, err := app.store.CartItems.GetItemByID(r.Context(), cart.ID, itemID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "cart item not found")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Delete the cart item
	err = app.store.CartItems.DeleteItem(r.Context(), cartItem.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Return a success response
	response := envelope{
		"message": "cart item deleted successfully",
	}
	app.successResponse(w, http.StatusOK, response)
}
