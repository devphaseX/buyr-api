package store

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/devphaseX/buyr-api.git/internal/filter"
	"github.com/devphaseX/buyr-api.git/internal/validator"
)

type PaginateQueryFilter struct {
	Page     int    `validate:"gte=1"`         // Page number (default: 1)
	PageSize int    `validate:"gte=1,lte=100"` // Number of items per page (default: 20)
	Sort     string // Sort field and direction (e.g., "created_at" or "-created_at")

	Filters      filter.Filterable
	SortSafelist []string // List of allowed sort fields
	*validator.Validator
}

// Parse extracts and converts query parameters from the HTTP request.
// It returns a PaginateQueryFilter and any conversion errors.
func (q *PaginateQueryFilter) Parse(r *http.Request) error {
	qs := r.URL.Query()
	q.Validator = validator.New()

	var filterValidationErrors validator.ValidationErrors

	// Parse and convert the "page" parameter
	page := qs.Get("page")
	if page != "" {
		p, err := strconv.Atoi(page)
		if err != nil {
			filterValidationErrors.AddFieldError("page", "must be an integer")
		} else {
			q.Page = p
		}
	}

	// Parse and convert the "page_size" parameter
	pageSize := qs.Get("page_size")
	if pageSize != "" {
		ps, err := strconv.Atoi(pageSize)
		if err != nil {
			filterValidationErrors.AddFieldError("page_size", "must be an integer")
		} else {
			q.PageSize = ps
		}
	}
	// Parse and validate the "sort" parameter
	sort := qs.Get("sort")
	if sort != "" {
		if !PermittedValue(sort, q.SortSafelist...) {
			filterValidationErrors.AddFieldError("sort", "invalid sort value")
		} else {
			q.Sort = sort
		}
	}

	// Validate the struct using the validator
	if err := q.Validator.Struct(q, &filterValidationErrors); err != nil {
		return err
	}

	if q.Filters != nil {
		if err := q.Filters.ParseFilters(r); err != nil {
			return err
		}

		if err := q.Validator.Struct(q.Filters, &filterValidationErrors); err != nil {
			return err
		}
	}

	return nil
}

// SortColumn returns the database column to sort by.
// It removes the "-" prefix (if any) to get the column name.
func (q *PaginateQueryFilter) SortColumn() string {
	for _, safeValue := range q.SortSafelist {
		if q.Sort == safeValue {
			return strings.TrimPrefix(q.Sort, "-")
		}
	}
	panic("unsafe sort parameter: " + q.Sort)
}

// SortDirection returns the sort direction ("ASC" or "DESC").
func (q *PaginateQueryFilter) SortDirection() string {
	if strings.HasPrefix(q.Sort, "-") {
		return "DESC"
	}
	return "ASC"
}

// Limit returns the number of items to return per page.
func (q *PaginateQueryFilter) Limit() int {
	return q.PageSize
}

// Offset returns the number of items to skip (for pagination).
func (q *PaginateQueryFilter) Offset() int {
	return (q.Page - 1) * q.PageSize
}

// PermittedValue checks if a value is in a list of permitted values.
func PermittedValue[T comparable](value T, permittedValues ...T) bool {
	for _, permittedValue := range permittedValues {
		if value == permittedValue {
			return true
		}
	}
	return false
}

type Metadata struct {
	CurrentPage  int `json:"current_page,omitempty"`
	PageSize     int `json:"page_size,omitempty"`
	FirstPage    int `json:"first_page,omitempty"`
	LastPage     int `json:"last_page,omitempty"`
	TotalRecords int `json:"total_records,omitempty"`
}

func calculateMetadata(totalRecords, page, pageSize int) Metadata {
	if totalRecords == 0 {
		return Metadata{}
	}

	return Metadata{
		CurrentPage:  page,
		PageSize:     pageSize,
		FirstPage:    1,
		LastPage:     int(math.Ceil(float64(totalRecords) / float64(pageSize))),
		TotalRecords: totalRecords,
	}
}
