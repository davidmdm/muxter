package muxter

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Middleware is a function that takes a handler and modifies its behaviour by returning a new handler
type Middleware = func(Handler) Handler

func WithMiddleware(handler Handler, middlewares ...Middleware) Handler {
	if handler == nil {
		return nil
	}
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// Recover allows you to register a handler function should a panic occur in the stack.
func Recover(recoverHandler func(recovered interface{}, w http.ResponseWriter, r *http.Request, c Context)) Middleware {
	return func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
			defer func() {
				if recovered := recover(); r != nil {
					recoverHandler(recovered, w, r, c)
					return
				}
			}()
			h.ServeHTTPx(w, r, c)
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

// CORS creates a middleware for enabling CORS with browsers.
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

	return func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
			if opts.AllowOriginFunc == nil && allowOrigin == "*" && opts.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
				w.Header().Add("Vary", "Origin")
			} else if opts.AllowOriginFunc != nil {
				w.Header().Set("Access-Control-Allow-Origin", opts.AllowOriginFunc(r.Header.Get("Origin")))
				w.Header().Add("Vary", "Origin") // Let browsers know that Access-Control-Allow-Origin varies by Origin
			} else {
				w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			}

			if opts.MaxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", strconv.Itoa(int(opts.MaxAge.Seconds())))
			}

			if opts.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if strings.ToUpper(r.Method) == "OPTIONS" {
				if allowHeaders != "" {
					w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
				} else {
					w.Header().Set("Access-Control-Allow-Headers", r.Header.Get("Access-Control-Request-Headers"))
					w.Header().Add("Vary", "Access-Control-Request-Headers")
				}

				w.Header().Set("Access-Control-Allow-Methods", allowMethods)

				w.WriteHeader(204)
				return
			}

			h.ServeHTTPx(w, r, c)
		})
	}
}

// DefaultCORS is a non restrictive configuration of the CORS middleware. It defaults to accepting
// any origin for CORS requests, and accepting any set of preflight request headers. It does not
// however default to AllowCredentials:true, therefore if making credentialed CORS requests you must
// configure this via the standard CORS middleware function.
var DefaultCORS = CORS(AccessControlOptions{})

// Decompress modifies the request body who's content-encoding is gzip with a gzip.ReadCloser that reads from the original
// source body. All readers are closed safely after the main handler returns.
var Decompress Middleware = func(h Handler) Handler {
	pool := sync.Pool{
		New: func() interface{} {
			return new(gzip.Reader)
		},
	}

	return HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
		if r.Header.Get("Content-Encoding") != "gzip" {
			h.ServeHTTPx(w, r, c)
			return
		}

		gr := pool.Get().(*gzip.Reader)
		defer pool.Put(gr)

		if err := gr.Reset(r.Body); err != nil {
			if errors.Is(err, io.EOF) {
				h.ServeHTTPx(w, r, c)
				return
			}
			http.Error(w, fmt.Sprintf("unexpected error: %v", err), 500)
			return
		}

		// Only close gzip reader if gr.Reset is successful otherwise decompressor is not set and close will panic.
		defer gr.Close()

		originalReqBody := r.Body
		defer originalReqBody.Close()

		r.Body = gr

		h.ServeHTTPx(w, r, c)
	})
}

// Skip decorates a middleware by giving it a predicate function for when this middleware should be skipped.
// if the predicateFunc returns true, the middleware is skipped.
func Skip(middleware Middleware, predicateFunc func(*http.Request) bool) Middleware {
	return func(h Handler) Handler {
		handlerWithMiddlewareApplied := middleware(h)

		return HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
			if predicateFunc(r) {
				h.ServeHTTPx(w, r, c)
				return
			}
			handlerWithMiddlewareApplied.ServeHTTPx(w, r, c)
		})
	}
}

type RespOverview struct {
	Request     *http.Request
	Response    http.ResponseWriter
	Context     Context
	Code        int
	TimeElapsed time.Duration
}

type responseProxy struct {
	http.ResponseWriter
	code int
}

func (w responseProxy) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *responseProxy) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

func (r responseProxy) Code() int {
	if r.code == 0 {
		return 200
	}
	return r.code
}

func Logger(dst io.Writer, fn func(overview RespOverview) string) Middleware {
	return func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
			proxy := responseProxy{w, 0}
			start := time.Now()

			h.ServeHTTPx(&proxy, r, c)

			fmt.Fprintln(dst, fn(RespOverview{
				Request:     r,
				Response:    w,
				Context:     c,
				Code:        proxy.Code(),
				TimeElapsed: time.Since(start),
			}))
		})
	}
}
