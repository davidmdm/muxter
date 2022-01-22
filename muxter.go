package muxter

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"
	"sync"
)

var _ http.Handler = &Mux{}

var defaultNotFoundHandler http.HandlerFunc = func(rw http.ResponseWriter, r *http.Request) {
	http.Error(rw, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

var defaultMethodNotAllowedHandler http.HandlerFunc = func(rw http.ResponseWriter, r *http.Request) {
	http.Error(rw, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

var redirectToSubdirHandler http.HandlerFunc = func(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Location", r.URL.Path+"/")
	rw.WriteHeader(http.StatusMovedPermanently)
}

type paramPool struct {
	pool *sync.Pool
}

func (p paramPool) Get() map[string]string {
	return p.pool.Get().(map[string]string)
}

func (p paramPool) Put(params map[string]string) {
	if params == nil {
		return
	}
	for k := range params {
		delete(params, k)
	}
	p.pool.Put(params)
}

var pool = paramPool{
	pool: &sync.Pool{New: func() interface{} { return make(map[string]string) }},
}

type node struct {
	wildcards      map[string]*node
	segments       map[string]*node
	fixedHandler   http.Handler
	subtreeHandler http.Handler
	mux            *Mux
}

func (n *node) lookup(url string) (targetMux *Mux, targetNode *node, params map[string]string, depth int) {
	var key string
	var subtreeNode *node
	var subtreeDepth int

	maxUrlLength := len(url)

	for {
		if n.mux != nil {
			m, node, p, d := n.mux.root.lookup(url)
			if node != nil {
				if m == nil {
					m = n.mux
				}
				return m, node, p, d + maxUrlLength - len(url)
			}
		}

		if n.subtreeHandler != nil {
			subtreeNode = n
			subtreeDepth = maxUrlLength - len(url)
		}

		key, url = split(url)
		if key == "" {
			break
		}

		if next, ok := n.segments[key]; ok {
			n = next
			continue
		}

		var param string
		max := -1

		for wildcard, wildNode := range n.wildcards {
			m, n, p, c := wildNode.lookup(url)
			if n != nil && c > max {
				targetMux, targetNode, params, max = m, n, p, c
				param = wildcard
			}
		}

		if targetNode == nil {
			break
		}

		if params == nil {
			params = pool.Get()
		}
		params[param] = key

		return targetMux, targetNode, params, maxUrlLength - len(url) + max
	}

	if key == "" {
		return nil, n, params, maxUrlLength
	}

	if subtreeNode != nil {
		return nil, subtreeNode, params, subtreeDepth
	}

	return nil, nil, nil, 0
}

func (n *node) traverse(pattern string) (target *node, remainder string) {
	var key string
	pattern = cleanPath(pattern)

	for {
		key, pattern = split(pattern)
		if key == "" {
			break
		}

		var nodeMap map[string]*node
		if key[0] == ':' {
			if n.wildcards == nil {
				n.wildcards = make(map[string]*node)
			}
			nodeMap = n.wildcards
			key = key[1:]
		} else {
			if n.segments == nil {
				n.segments = make(map[string]*node)
			}
			nodeMap = n.segments
		}

		next, ok := nodeMap[key]
		if !ok {
			next = new(node)
			nodeMap[key] = next
		}
		n = next
	}

	return n, pattern
}

func (n *node) clone(middlewares ...Middleware) *node {
	clone := new(node)

	clone.mux = n.mux
	if clone.mux != nil {
		clone.mux = clone.mux.clone(middlewares...)
	}

	if n.fixedHandler != nil {
		clone.fixedHandler = WithMiddleware(n.fixedHandler, middlewares...)
	}

	if n.subtreeHandler != nil {
		clone.subtreeHandler = WithMiddleware(n.subtreeHandler, middlewares...)
	}

	if n.segments != nil {
		clone.segments = make(map[string]*node)
		for key, value := range n.segments {
			clone.segments[key] = value.clone(middlewares...)
		}
	}

	if n.wildcards != nil {
		clone.wildcards = make(map[string]*node)
		for key, value := range n.wildcards {
			clone.wildcards[key] = value.clone(middlewares...)
		}
	}

	return clone
}

// Mux is a request multiplexer with the same routing behaviour as the standard libraries net/http ServeMux
type Mux struct {
	root               node
	NotFoundHandler    http.HandlerFunc
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
	m := &Mux{}
	for _, apply := range options {
		apply(m)
	}
	return m
}

// ServeHTTP implements the net/http Handler interface.
func (m *Mux) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	path := cleanPath(req.URL.Path)
	targetMux, node, params, length := m.root.lookup(path)
	if targetMux == nil {
		targetMux = m
	}
	defer pool.Put(params)

	trailingSlash := strings.HasSuffix(path, "/")

	var handler http.Handler

	if node != nil {
		if length < len(path) {
			handler = node.subtreeHandler
		} else if trailingSlash {
			if node.subtreeHandler != nil {
				handler = node.subtreeHandler
			} else if targetMux.matchTrailingSlash && node.fixedHandler != nil {
				handler = node.fixedHandler
			}
		} else if node.fixedHandler != nil {
			handler = node.fixedHandler
		} else if node.subtreeHandler != nil {
			handler = redirectToSubdirHandler
		}
	}

	if handler == nil {
		if targetMux.NotFoundHandler != nil {
			handler = targetMux.NotFoundHandler
		} else {
			handler = defaultNotFoundHandler
		}
	}

	if params != nil {
		*req = *req.WithContext(context.WithValue(req.Context(), paramKey, params))
	}

	handler.ServeHTTP(res, req)
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

	node, remainder := m.root.traverse(pattern)

	if remainder == "/" {
		node.subtreeHandler = handler
	} else {
		node.fixedHandler = handler
	}
}

// RegisterMux registers a mux at a given pattern. This allows for mux's to be composed.
// Middlewares are called in this order: parent global middlewares, middlewares passed here, child mux global middlewares and child middlewares.
func (m *Mux) RegisterMux(pattern string, mux *Mux, middlewares ...Middleware) {
	n, _ := m.root.traverse(pattern)
	n.mux = mux.clone(concatMiddlewares(m.middlewares, middlewares)...)
}

func (m *Mux) clone(middlewares ...Middleware) *Mux {
	middlewares = concatMiddlewares(middlewares, m.middlewares)
	clone := &Mux{
		root:               *m.root.clone(middlewares...),
		middlewares:        middlewares,
		NotFoundHandler:    m.NotFoundHandler,
		matchTrailingSlash: m.matchTrailingSlash,
	}
	return clone
}

type paramKeyType int

var paramKey paramKeyType

// Param reads path params from the request
func Param(r *http.Request, param string) string {
	if r == nil {
		return ""
	}

	if params, ok := r.Context().Value(paramKey).(map[string]string); ok && params != nil {
		return params[param]
	}

	return ""
}

// Params returns all path params in a map. Prefer the simple Param to avoid memory allocations.
func Params(r *http.Request) map[string]string {
	if r == nil {
		return nil
	}

	params, _ := r.Context().Value(paramKey).(map[string]string)
	if params == nil {
		return nil
	}

	// The params map belongs to a pool and will be put back and cleared once ServeHTTP is done.
	// Should a user capture the map in a variable that outlives the lifetime of the handler, it
	// would be very hard for them to understand where their params have gone. Hence return a copy
	// of the params.
	cpy := make(map[string]string)
	for k, v := range params {
		cpy[k] = v
	}

	return cpy
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

func split(pattern string) (head, rest string) {
	if pattern == "" {
		return "", ""
	}

	if pattern == "/" {
		return "", "/"
	}

	if pattern[0] == '/' {
		pattern = pattern[1:]
	}

	var idx int
	for _, b := range []byte(pattern) {
		if b == '/' {
			break
		}
		idx++
	}

	return pattern[:idx], pattern[idx:]
}

type MethodHandler struct {
	handlers                map[string]http.Handler
	methodNotAllowedHandler http.Handler
}

type MethodHandlerMap = map[string]http.Handler

// MakeMethodHandler takes a map of http verbs to http handlers and a handler should no method match.
// If nil is provided for the methodNotAllowedHandler the default handler will be used.
func MakeMethodHandler(handlerMap MethodHandlerMap, methodNotAllowedHandler http.Handler) MethodHandler {
	handlers := make(map[string]http.Handler)
	for method, handler := range handlerMap {
		handlers[strings.ToUpper(method)] = handler
	}

	if methodNotAllowedHandler == nil {
		methodNotAllowedHandler = defaultMethodNotAllowedHandler
	}

	return MethodHandler{
		handlers:                handlers,
		methodNotAllowedHandler: methodNotAllowedHandler,
	}
}

func (mh MethodHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if h := mh.handlers[strings.ToUpper(r.Method)]; h != nil {
		h.ServeHTTP(rw, r)
		return
	}

	if h := mh.methodNotAllowedHandler; h != nil {
		h.ServeHTTP(rw, r)
		return
	}

	defaultMethodNotAllowedHandler(rw, r)
}

func concatMiddlewares(stacks ...[]Middleware) []Middleware {
	fmt.Println("YOLO", stacks)
	var middlewares []Middleware
	for _, stack := range stacks {
		middlewares = append(middlewares, stack...)
	}
	return middlewares
}
