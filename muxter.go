package muxter

import (
	"context"
	"net/http"
	"path"
	"strings"
	"sync"
)

var _ http.Handler = Mux{}

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
	pool: &sync.Pool{
		New: func() interface{} {
			return make(map[string]string)
		},
	},
}

type node struct {
	wildcards      map[string]*node
	segments       map[string]*node
	fixedHandler   http.Handler
	subtreeHandler http.Handler
}

func (n *node) lookup(url string) (handler http.Handler, params map[string]string, depth int) {
	var key string
	var subtreeHandler http.Handler
	var subtreeDepth int

	maxUrlLength := len(url)

	for {
		if n.subtreeHandler != nil {
			subtreeHandler = n.subtreeHandler
			subtreeDepth = maxUrlLength - len(url)
		}

		key, url = split(url)
		if key != "" {
			if next, ok := n.segments[key]; ok {
				n = next
				continue
			}

			var handler http.Handler
			var param string
			max := -1

			for wildcard, wildNode := range n.wildcards {
				h, p, c := wildNode.lookup(url)
				if h != nil && c > max {
					handler, params, max = h, p, c
					param = wildcard
				}
			}

			if handler != nil {
				if params == nil {
					params = pool.Get()
				}
				params[param] = key

				return handler, params, maxUrlLength - len(url) + max
			}

			if subtreeHandler != nil {
				return subtreeHandler, params, subtreeDepth
			}

			return nil, nil, 0
		}

		if url == "" {
			if n.fixedHandler != nil {
				return n.fixedHandler, params, maxUrlLength
			}
			if h, _, _ := n.lookup("/"); h != nil {
				return redirectToSubdirHandler, params, maxUrlLength
			}
		}

		if subtreeHandler != nil {
			return subtreeHandler, params, subtreeDepth
		}

		return nil, nil, 0
	}
}

func (n *node) merge(other *node, middlewares ...Middleware) *node {
	if other.fixedHandler != nil {
		n.fixedHandler = WithMiddleware(other.fixedHandler, middlewares...)
	}

	if other.subtreeHandler != nil {
		n.subtreeHandler = WithMiddleware(other.subtreeHandler, middlewares...)
	}

	for segment, nextNode := range other.segments {
		if n.segments == nil {
			n.segments = make(map[string]*node)
		}
		if currentNode := n.segments[segment]; currentNode == nil {
			n.segments[segment] = new(node).merge(nextNode, middlewares...)
		} else {
			currentNode.merge(nextNode, middlewares...)
		}
	}

	for wildcard, nextNode := range other.wildcards {
		if n.wildcards == nil {
			n.wildcards = make(map[string]*node)
		}
		if currentNode := n.wildcards[wildcard]; currentNode == nil {
			n.wildcards[wildcard] = new(node).merge(nextNode, middlewares...)
		} else {
			currentNode.merge(nextNode)
		}
	}

	return n
}

// Mux is a request multiplexer with the same routing behaviour as the standard libraries net/http ServeMux
type Mux struct {
	root        node
	middlewares []Middleware

	// NotFoundHandler will be called if no registered route has been matched for a given request.
	// if NotFoundHandler is nil a default NotFoundHandler will be used instead simply returning 404 and the default http.StatusText(404) as body.
	NotFoundHandler http.HandlerFunc
}

// New returns a pointer to a new muxter.Mux
func New() *Mux {
	return &Mux{}
}

// ServeHTTP implements the net/http Handler interface.
func (m Mux) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	handler, params, _ := m.root.lookup(cleanPath(req.URL.Path))
	defer pool.Put(params)

	if handler == nil {
		if m.NotFoundHandler != nil {
			handler = m.NotFoundHandler
		} else {
			handler = defaultNotFoundHandler
		}
	}

	if params != nil {
		ctx := context.WithValue(req.Context(), paramKey, params)
		req = req.WithContext(ctx)
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
	n.merge(&mux.root, append(m.middlewares, middlewares...)...)
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
