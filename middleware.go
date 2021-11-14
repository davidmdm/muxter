package muxter

import (
	"net/http"
	"strconv"
	"strings"
	"time"
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

// AccessControlOptions provides options for the CORS middleware.
type AccessControlOptions struct {
	// AllowOrigin is the origin that is set for the Access-Control-Allow-Origin. If it is "*" and
	// AllowCredentials is true the incoming Origin will be used.
	AllowOrigin string

	// AllowOriginFunc takes the request origin and returns the Access-Control-Allow-Origin.
	// Takes precedence over AllowOrigin.
	AllowOriginFunc func(origin string) string

	// MaxAge sets the Access-Control-Max-Age property.
	MaxAge time.Duration

	AllowCredentials bool
	ExposeHeaders    []string
	AllowHeaders     []string
	AllowMethods     []string
}

func CORS(opts AccessControlOptions) Middleware {
	if opts.AllowOrigin == "" {
		opts.AllowOrigin = "*"
	}
	allowOrigin := opts.AllowOrigin

	if opts.AllowMethods == nil {
		opts.AllowMethods = []string{"GET", "POST", "HEAD", "PUT", "PATCH", "DELETE"}
	}
	allowMethods := strings.Join(opts.AllowMethods, ", ")
	allowHeaders := strings.Join(opts.AllowHeaders, ", ")

	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			if opts.AllowOriginFunc == nil && allowOrigin == "*" && opts.AllowCredentials {
				rw.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
				rw.Header().Add("Vary", "Origin")
			} else if opts.AllowOriginFunc != nil {
				rw.Header().Set("Access-Control-Allow-Origin", opts.AllowOriginFunc(r.Header.Get("Origin")))
				rw.Header().Add("Vary", "Origin") // Let browsers know that Access-Control-Allow-Origin varies by Origin
			} else {
				rw.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			}

			if opts.MaxAge > 0 {
				rw.Header().Set("Access-Control-Max-Age", strconv.Itoa(int(opts.MaxAge.Seconds())))
			}

			if opts.AllowCredentials {
				rw.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if r.Method == "OPTIONS" {
				if allowHeaders != "" {
					rw.Header().Set("Access-Control-Allow-Headers", allowHeaders)
				} else {
					rw.Header().Set("Access-Control-Allow-Headers", r.Header.Get("Access-Control-Request-Headers"))
					rw.Header().Add("Vary", "Access-Control-Request-Headers")
				}

				rw.Header().Set("Access-Control-Allow-Methods", allowMethods)

				rw.WriteHeader(204)
				return
			}

			h.ServeHTTP(rw, r)
		})
	}
}
