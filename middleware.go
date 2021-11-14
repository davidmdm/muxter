package muxter

import (
	"net/http"
	"strings"
)

// Middleware is a function that takes a handler and modifies its behaviour by returning a new handler
type Middleware = func(http.Handler) http.Handler

func withMiddleware(handler http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// Method takes an method string as an argument and returns a middleware that checks if the request method
// matches the provided method. If the check fails a 405 is returned. The check is case insensitive.
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

func Recover(recoverHandler func(recovered interface{}, rw http.ResponseWriter, r *http.Request)) Middleware {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); r != nil {
					recoverHandler(recovered, rw, r)
					return
				}
			}()
			h.ServeHTTP(rw, r)
		})
	}
}
