package httpapi

import "net/http"

func NewRouter(handler *Handler, corsAllowOrigin string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handler.Healthz)
	mux.HandleFunc("/readyz", handler.Readyz)
	mux.HandleFunc("/v1/listings/count", handler.ListingsCount)
	mux.HandleFunc("/v1/changes", handler.Changes)
	mux.HandleFunc("/v1/listings/", handler.ListingsByKey)
	mux.HandleFunc("/v1/listings", handler.ListingsBrowse)
	return withCORS(corsAllowOrigin, mux)
}
