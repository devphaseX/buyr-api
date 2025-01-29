package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/devphaseX/buyr-api.git/internal/store"
)

type CreateProductImageRequest struct {
	URL       string `json:"url" validate:"required,url"`
	IsPrimary bool   `json:"is_primary"`
}

// Request struct for creating a product feature
type CreateProductFeatureRequest struct {
	Title          string                   `json:"title" validate:"required,max=255"`
	View           store.ProductFeatureView `json:"view" validate:"required,max=255"`
	FeatureEntries map[string]interface{}   `json:"feature_entries" validate:"required"`
}

type CreateProductRequest struct {
	Name           string                        `json:"name" validate:"required,max=255"`
	Description    string                        `json:"description" validate:"required"`
	StockQuantity  int                           `json:"stock_quantity"`
	Discount       float64                       `json:"discount"`
	PrimaryImageID int                           `json:"primary_image_id"`
	Price          float64                       `json:"price" validate:"required"`
	CategoryID     string                        `json:"category_id" validate:"required"`
	Images         []CreateProductImageRequest   `json:"images" validate:"required,dive"`
	Features       []CreateProductFeatureRequest `json:"features" validate:"required,dive"`
}

func (app *application) createProduct(w http.ResponseWriter, r *http.Request) {
	var form CreateProductRequest

	if err := app.readJSON(w, r, &form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := validate.Struct(form); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := getUserFromCtx(r)

	vendorUser, err := app.store.Users.GetVendorUserByID(r.Context(), user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	product := &store.Product{
		Name:          form.Name,
		Description:   form.Description,
		StockQuantity: form.StockQuantity,
		VendorID:      vendorUser.ID,
		Discount:      form.Discount,
		Price:         form.Price,
		CategoryID:    form.CategoryID,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	var productImages []*store.ProductImage

	for _, image := range form.Images {
		productImages = append(productImages, &store.ProductImage{
			URL:       image.URL,
			IsPrimary: image.IsPrimary,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}

	product.Images = productImages

	// Map request data to ProductFeature database models
	var productFeatures []*store.ProductFeature
	for _, featureReq := range form.Features {
		productFeatures = append(productFeatures, &store.ProductFeature{
			Title:          featureReq.Title,
			View:           featureReq.View,
			FeatureEntries: featureReq.FeatureEntries,
			ProductID:      product.ID,
		})
	}

	product.Features = productFeatures

	if err := app.store.Products.Create(r.Context(), product); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{
		"product": product,
	}

	app.successResponse(w, http.StatusCreated, response)
}

func (app *application) publishProduct(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	vendorUser, err := app.store.Users.GetVendorUserByID(r.Context(), user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	productID := app.readStringID(r, "productID")
	if err := app.store.Products.Publish(r.Context(), productID, vendorUser.ID); err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "product not found")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	response := envelope{
		"message": "product published successfully",
		"id":      productID,
	}
	// Return success response
	app.successResponse(w, http.StatusOK, response)
}

func (app *application) unPublishProduct(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	vendorUser, err := app.store.Users.GetVendorUserByID(r.Context(), user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	productID := app.readStringID(r, "productID")
	if err := app.store.Products.Unpublish(r.Context(), productID, vendorUser.ID); err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "product not found")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	response := envelope{
		"message": "product unpublished successfully",
		"id":      productID,
	}
	// Return success response
	app.successResponse(w, http.StatusOK, response)
}

func (app *application) approveProduct(w http.ResponseWriter, r *http.Request) {
	productID := app.readStringID(r, "productID")

	if err := app.store.Products.Approve(r.Context(), productID); err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "product not found")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	response := envelope{
		"message": "product unpublished successfully",
		"id":      productID,
	}
	// Return success response
	app.successResponse(w, http.StatusOK, response)
}

func (app *application) rejectProduct(w http.ResponseWriter, r *http.Request) {
	productID := app.readStringID(r, "productID")

	if err := app.store.Products.Reject(r.Context(), productID); err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "product not found")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	response := envelope{
		"message": "product unpublished successfully",
		"id":      productID,
	}
	// Return success response
	app.successResponse(w, http.StatusOK, response)
}

func (app *application) getProduct(w http.ResponseWriter, r *http.Request) {
	productID := app.readStringID(r, "productID")

	// Fetch the product, images, and features using a single query with LEFT JOIN
	product, err := app.store.Products.GetWithDetails(r.Context(), productID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRecordNotFound):
			app.notFoundResponse(w, r, "product not found")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Construct the final response
	response := envelope{
		"product": product,
	}

	// Return the response
	app.successResponse(w, http.StatusOK, response)
}

func (app *application) getProducts(w http.ResponseWriter, r *http.Request) {
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

	products, metadata, err := app.store.Products.GetProducts(r.Context(), fq)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.successResponse(w, http.StatusOK, envelope{
		"products": products,
		"metadata": metadata,
	})
}
