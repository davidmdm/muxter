package muxter

import (
	"net/http"
	"strings"
)

type Middleware func(http.Handler) http.Handler

func withMiddleware(handler http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

func Method(method string) Middleware {
	method = strings.ToUpper(method)
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			if method != strings.ToUpper(r.Method) {
				http.Error(rw, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
				return
			}
			h.ServeHTTP(rw, r)
		})
	}
}

var (
	GET    = Method("GET")
	POST   = Method("POST")
	PATCH  = Method("PATCH")
	PUT    = Method("PUT")
	DELETE = Method("DELETE")
)
