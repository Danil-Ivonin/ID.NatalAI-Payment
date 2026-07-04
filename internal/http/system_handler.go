package http

import "net/http"

func NewHealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeNotImplemented(w)
	})
}

func NewReadyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeNotImplemented(w)
	})
}

func handlerOrNotImplemented(handler http.Handler) http.Handler {
	if handler != nil {
		return handler
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeNotImplemented(w)
	})
}

func writeNotImplemented(w http.ResponseWriter) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
