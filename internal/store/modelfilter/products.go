package modelfilter

import "net/http"

type GetProductsFilter struct {
	VendorID  *string
	AdminView bool
}

func (f *GetProductsFilter) ParseFilters(r *http.Request) error {
	query := r.URL.Query()

	vendorID := query.Get("vendor_id")

	if f.VendorID == nil && len(vendorID) > 0 {
		f.VendorID = &vendorID
	}

	return nil
}
