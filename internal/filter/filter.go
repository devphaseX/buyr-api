package filter

import "net/http"

type Filterable interface {
	ParseFilters(r *http.Request) error
}
