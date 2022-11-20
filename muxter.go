package muxter

import (
	"fmt"
	"net/http"
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
	root               *node
	notFoundHandler    Handler
	middlewares        []Middleware
	matchTrailingSlash bool
}

type MuxOption func(*Mux)

// MatchTrailingSlash will allow a fixed handler to match a route with an inbound trailing slash
// if no rooted subtree handler is registered at that route.
func MatchTrailingSlash(value bool) MuxOption {
	return func(m *Mux) {
		m.matchTrailingSlash = value
	}
}

// New returns a pointer to a new muxter.Mux
func New(options ...MuxOption) *Mux {
	m := &Mux{
		root:               &node{},
		notFoundHandler:    defaultNotFoundHandler,
		middlewares:        []Middleware{},
		matchTrailingSlash: false,
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
	value := m.root.Lookup(r.URL.Path, c.params, m.matchTrailingSlash)

	var handler Handler
	if value != nil {
		handler = value.handler
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
	}

	handler.ServeHTTPx(w, r, c)
}

func (m *Mux) SetNotFoundHandler(handler Handler) {
	m.notFoundHandler = WithMiddleware(handler, m.middlewares...)
}

func (m *Mux) SetNotFoundHandlerFunc(handler HandlerFunc) {
	m.SetNotFoundHandler(handler)
}

// Use registers global middlewares for your routes. Only routes registered after the call to use will be affected
// by a call to Use. Middlewares will be invoked such that the first middleware will have its effect run before the second
// and so forth.
func (m *Mux) Use(middlewares ...Middleware) {
	m.middlewares = append(m.middlewares, middlewares...)
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

	handler = WithMiddleware(handler, append(m.middlewares, middlewares...)...)
	if err := m.root.Insert(pattern, &value{handler: handler, pattern: pattern}); err != nil {
		panic(fmt.Sprintf("muxter: failed to register route %s - %v", pattern, err))
	}
}

func (m *Mux) StandardHandle(pattern string, handler http.Handler, middlewares ...Middleware) {
	m.Handle(pattern, Adaptor(handler))
}

type MethodHandler struct {
	GET                     Handler
	POST                    Handler
	PUT                     Handler
	PATCH                   Handler
	HEAD                    Handler
	DELETE                  Handler
	MethodNotAllowedHandler Handler
}

func (mh MethodHandler) getHandler(method string) (handler Handler) {
	defer func() {
		if handler == nil {
			if mh.MethodNotAllowedHandler == nil {
				handler = defaultMethodNotAllowedHandler
			} else {
				handler = mh.MethodNotAllowedHandler
			}
		}
	}()

	switch strings.ToUpper(method) {
	case "GET":
		return mh.GET
	case "POST":
		return mh.POST
	case "DELETE":
		return mh.DELETE
	case "PUT":
		return mh.PUT
	case "PATCH":
		return mh.PATCH
	case "HEAD":
		return mh.HEAD
	default:
		return nil
	}
}

func (mh MethodHandler) ServeHTTPx(w http.ResponseWriter, r *http.Request, c Context) {
	mh.getHandler(r.Method).ServeHTTPx(w, r, c)
}
