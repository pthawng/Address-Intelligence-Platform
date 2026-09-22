package httpserver

import (
	"fmt"
	"net/http"
)

type PaginationMeta struct {
	Page         int64 `json:"page"`
	Limit        int64 `json:"limit"`
	TotalRecords int64 `json:"total_records"`
	TotalPages   int64 `json:"total_pages"`
}

// Paginated uses one-based pages. An empty collection has zero total pages.
// A page beyond the last page is permitted and must contain no items.
func Paginated[T any](w http.ResponseWriter, r *http.Request, items []T, page, limit, total int64) error {
	if page < 1 || limit < 1 || total < 0 {
		return fmt.Errorf("invalid pagination metadata")
	}
	pages := total / limit
	if total%limit != 0 {
		pages++
	} // avoids overflow in total+limit-1
	count := int64(len(items))
	if count > limit || count > total || (page > pages && count != 0) {
		return fmt.Errorf("items contradict pagination metadata")
	}
	if page == pages && total%limit != 0 && count > total%limit {
		return fmt.Errorf("items exceed final page size")
	}
	if items == nil {
		items = []T{}
	}
	return writeJSON(w, r, http.StatusOK, SuccessResponse{
		Success: true, Status: http.StatusOK, Data: items,
		Meta: &PaginationMeta{Page: page, Limit: limit, TotalRecords: total, TotalPages: pages},
	})
}
