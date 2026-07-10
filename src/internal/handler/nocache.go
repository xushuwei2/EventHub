package handler

import "net/http"

func NoCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setNoCacheHeaders(w)
		next.ServeHTTP(w, r)
	})
}

func setNoCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "private, no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("CDN-Cache-Control", "no-store")
	w.Header().Set("Surrogate-Control", "no-store")
}
