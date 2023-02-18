package muxter

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/davidmdm/muxter/internal/pool"
)

var _ http.Handler = &Mux{}

var defaultNotFoundHandler HandlerFunc = func(w http.ResponseWriter, r *http.Request, c Context) {
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

var defaultRedirectHandler HandlerFunc = func(w http.ResponseWriter, r *http.Request, c Context) {
	w.Header().Set("Location", c.ogReqPath+"/")
	w.WriteHeader(http.StatusMovedPermanently)
}

var defaultMethodNotAllowedHandler HandlerFunc = func(w http.ResponseWriter, r *http.Request, c Context) {
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

// Mux is a request multiplexer with the same routing behaviour as the standard libraries net/http ServeMux
type Mux struct {
	notFoundHandler         Handler
	methodNotAllowedHandler Handler
	root                    *node
	matchTrailingSlash      *bool
	middlewares             []Middleware
	globalwares             []Middleware
}

type MuxOption func(*Mux)

// MatchTrailingSlash will allow a fixed handler to match a route with an inbound trailing slash
// if no rooted subtree handler is registered at that route.
func MatchTrailingSlash(value bool) MuxOption {
	return func(m *Mux) {
		m.matchTrailingSlash = &value
	}
}

// New returns a pointer to a new muxter.Mux
func New(options ...MuxOption) *Mux {
	m := &Mux{
		root:               &node{},
		middlewares:        []Middleware{},
		globalwares:        []Middleware{},
		notFoundHandler:    nil,
		matchTrailingSlash: nil,
	}
	for _, apply := range options {
		apply(m)
	}
	return m
}

// ServeHTTP implements the net/http Handler interface.
func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := Context{
		ogReqPath: r.URL.Path,
		params:    pool.Params.Get(),
	}
	m.ServeHTTPx(w, r, c)
	pool.Params.Put(c.params)
}

func (m *Mux) ServeHTTPx(w http.ResponseWriter, r *http.Request, c Context) {
	value := m.root.Lookup(r.URL.Path, c.params, m.matchTrailingSlash != nil && *m.matchTrailingSlash)

	var handler Handler
	if value != nil {
		if value.isRedirect {
			handler = WithMiddleware(defaultRedirectHandler, m.globalwares...)
		} else {
			handler = value.handler
		}
		if c.pattern != "" {
			c.pattern = c.pattern + value.pattern[1:]
		} else {
			c.pattern = value.pattern
		}
	} else {
		if m.notFoundHandler != nil {
			handler = m.notFoundHandler
		} else {
			handler = defaultNotFoundHandler
		}
		handler = WithMiddleware(handler, m.globalwares...)
	}

	handler.ServeHTTPx(w, r, c)
}

func (m *Mux) SetNotFoundHandler(handler Handler) {
	m.notFoundHandler = handler
}

func (m *Mux) SetNotFoundHandlerFunc(handler HandlerFunc) {
	m.SetNotFoundHandler(handler)
}

func (m *Mux) SetMethodNotAllowedHandler(handler Handler) {
	m.methodNotAllowedHandler = handler
}

func (m *Mux) SetMethodNotAllowedHandlerFunc(handler HandlerFunc) {
	m.SetMethodNotAllowedHandler(handler)
}

// Use registers global middlewares for your routes. Only routes registered after the call to use will be affected
// by a call to Use. Middlewares will be invoked such that the first middleware will have its effect run before the second
// and so forth. Middlewares are not executed for globally set behavior like redirects or route not found. For middlewares
// that will be include those routes see useGlobal
func (m *Mux) Use(middlewares ...Middleware) {
	m.middlewares = append(m.middlewares, middlewares...)
}

// UseGlobal registers middlewares globally. A global middleware is registered for normally like a call to Use(),
// the only difference is that all globally registered middlewares will be applied to the not found and redirect handlers.
// UseGlobal is best used near the beginning and for concerns like logging and tracing.
func (m *Mux) UseGlobal(middlewares ...Middleware) {
	m.middlewares = append(m.middlewares, middlewares...)
	m.globalwares = append(m.globalwares, middlewares...)
}

// HandleFunc registers a net/http HandlerFunc for a given string pattern. Middlewares are applied
// such that the first middleware will be called before passing control to the next middleware.
// ie mux.HandleFunc(pattern, handler, m1, m2, m3) => request flow will pass through m1 then m2 then m3.
func (m *Mux) HandleFunc(pattern string, handler HandlerFunc, middlewares ...Middleware) {
	m.Handle(pattern, handler, middlewares...)
}

// Handle registers a net/http HandlerFunc for a given string pattern. Middlewares are applied
// such that the first middleware will be called before passing control to the next middleware.
// ie mux.HandleFunc(pattern, handler, m1, m2, m3) => request flow will pass through m1 then m2 then m3.
func (m *Mux) Handle(pattern string, handler Handler, middlewares ...Middleware) {
	if pattern == "" {
		panic("muxter: cannot register empty route pattern")
	}
	if pattern[0] != '/' {
		panic("muxter: route pattern must begin with a forward-slash: '/' but got: " + pattern)
	}
	if handler == nil {
		panic("muxter: handler cannot be nil")
	}

	if mh, ok := handler.(*Mux); ok {
		cpy := *mh
		if cpy.notFoundHandler == nil {
			cpy.notFoundHandler = m.notFoundHandler
		}
		if cpy.matchTrailingSlash == nil {
			cpy.matchTrailingSlash = m.matchTrailingSlash
		}
		if cpy.methodNotAllowedHandler == nil {
			cpy.methodNotAllowedHandler = m.methodNotAllowedHandler
		}
		cpy.globalwares = append(append([]Middleware{}, m.globalwares...), cpy.globalwares...)
		handler = &cpy
	}

	handler = WithMiddleware(handler, append(m.middlewares, middlewares...)...)
	if err := m.root.Insert(pattern, &value{handler: handler, pattern: pattern}); err != nil {
		panic(fmt.Sprintf("muxter: failed to register route %s - %v", pattern, err))
	}
}

func (m *Mux) StandardHandle(pattern string, handler http.Handler, middlewares ...Middleware) {
	m.Handle(pattern, Adaptor(handler))
}

func (m *Mux) Method(method string) Middleware {
	methodNotAllowed := m.methodNotAllowedHandler
	if methodNotAllowed == nil {
		methodNotAllowed = defaultMethodNotAllowedHandler
	}

	method = strings.ToUpper(method)

	return func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
			if strings.ToUpper(r.Method) != method {
				methodNotAllowed.ServeHTTPx(w, r, c)
				return
			}
			h.ServeHTTPx(w, r, c)
		})
	}
}

func (m *Mux) GET() Middleware {
	return func(h Handler) Handler {
		getGuard := m.Method("GET")(h)
		headGuard := m.HEAD()(h)
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
			if strings.ToUpper(r.Method) == "HEAD" {
				headGuard.ServeHTTPx(w, r, c)
				return
			}
			getGuard.ServeHTTPx(w, r, c)
		})
	}
}

func (m *Mux) HEAD() Middleware {
	return func(h Handler) Handler {
		guard := m.Method("HEAD")(h)
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request, c Context) {
			if strings.ToUpper(r.Method) != "HEAD" {
				guard.ServeHTTPx(w, r, c)
				return
			}

			hrw := &headResponseWriter{w, 0}
			h.ServeHTTPx(hrw, r, c)
			if w.Header().Get("Content-Length") == "" {
				w.Header().Set("Content-Length", strconv.Itoa(hrw.contentLength))
			}
		})
	}
}

func (m *Mux) POST() Middleware   { return m.Method("POST") }
func (m *Mux) PUT() Middleware    { return m.Method("PUT") }
func (m *Mux) PATCH() Middleware  { return m.Method("PATCH") }
func (m *Mux) DELETE() Middleware { return m.Method("DELETE") }

type headResponseWriter struct {
	http.ResponseWriter
	contentLength int
}

func (w headResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *headResponseWriter) Write(b []byte) (int, error) {
	w.contentLength += len(b)
	return len(b), nil
}
