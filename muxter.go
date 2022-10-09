package muxter

import (
	"context"
	"net/http"
	"path"
	"strings"

	tree "github.com/davidmdm/muxter/internal"
)

var _ http.Handler = &Mux{}

var defaultNotFoundHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

var defaultMethodNotAllowedHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

var redirectToSubdirHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Location", r.URL.Path+"/")
	w.WriteHeader(http.StatusMovedPermanently)
}

// Mux is a request multiplexer with the same routing behaviour as the standard libraries net/http ServeMux
type Mux struct {
	root               *tree.Node
	notFoundHandler    http.Handler
	middlewares        []Middleware
	matchTrailingSlash bool
}

type MuxOption func(*Mux)

func MatchTrailingSlash(value bool) MuxOption {
	return func(m *Mux) {
		m.matchTrailingSlash = value
	}
}

// New returns a pointer to a new muxter.Mux
func New(options ...MuxOption) *Mux {
	m := &Mux{
		root:               &tree.Node{},
		notFoundHandler:    defaultNotFoundHandler,
		middlewares:        []func(http.Handler) http.Handler{},
		matchTrailingSlash: false,
	}
	for _, apply := range options {
		apply(m)
	}
	return m
}

// ServeHTTP implements the net/http Handler interface.
func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := cleanPath(r.URL.Path)

	params, _ := r.Context().Value(paramKey).(map[string]string)
	shouldInjectParams := params == nil

	if params == nil {
		params = pool.Get()
		defer pool.Put(params)
	}

	node, params := m.root.Lookup(path, params)

	var handler http.Handler
	if node == nil || node.Handler == nil {
		if m.notFoundHandler == nil {
			handler = defaultNotFoundHandler
		} else {
			handler = m.notFoundHandler
		}
	} else {
		handler = node.Handler
	}

	if shouldInjectParams {
		*r = *r.WithContext(context.WithValue(r.Context(), paramKey, params))
	}

	handler.ServeHTTP(w, r)
}

func (m *Mux) SetNotFoundHandler(handler http.Handler) {
	m.notFoundHandler = WithMiddleware(handler, m.middlewares...)
}

func (m *Mux) SetNotFoundHandlerFunc(handler http.HandlerFunc) {
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
func (m *Mux) HandleFunc(pattern string, handler http.HandlerFunc, middlewares ...Middleware) {
	m.Handle(pattern, handler, middlewares...)
}

// Handle registers a net/http HandlerFunc for a given string pattern. Middlewares are applied
// such that the first middleware will be called before passing control to the next middleware.
// ie mux.HandleFunc(pattern, handler, m1, m2, m3) => request flow will pass through m1 then m2 then m3.
func (m *Mux) Handle(pattern string, handler http.Handler, middlewares ...Middleware) {
	handler = WithMiddleware(handler, append(m.middlewares, middlewares...)...)
	m.root.Insert(pattern, handler)
}

// Taken from standard library: package net/http.
// cleanPath returns the canonical path for p, eliminating . and .. elements.
func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	np := path.Clean(p)
	// path.Clean removes trailing slash except for root;
	// put the trailing slash back if necessary.
	if p[len(p)-1] == '/' && np != "/" {
		// Fast path for common case of p being the string we want:
		if len(p) == len(np)+1 && strings.HasPrefix(p, np) {
			np = p
		} else {
			np += "/"
		}
	}
	return np
}

type MethodHandler struct {
	GET                     http.Handler
	POST                    http.Handler
	PUT                     http.Handler
	PATCH                   http.Handler
	HEAD                    http.Handler
	DELETE                  http.Handler
	MethodNotAllowedHandler http.Handler
}

func (mh MethodHandler) getHandler(method string) (handler http.Handler) {
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

func (mh MethodHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mh.getHandler(r.Method).ServeHTTP(w, r)
}
