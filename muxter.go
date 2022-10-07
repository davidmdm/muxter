package muxter

import (
	"context"
	"net/http"
	"path"
	"strings"
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

type node struct {
	wildcards      map[string]*node
	segments       map[string]*node
	fixedHandler   http.Handler
	subtreeHandler http.Handler
}

func (n *node) lookup(url string, p map[string]string) (targetNode *node, params map[string]string, depth int) {
	var key string
	var subtreeNode *node
	var subtreeDepth int

	params = p

	maxUrlLength := len(url)

	for {
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
			n, p, c := wildNode.lookup(url, params)
			if n != nil && c > max {
				targetNode, params, max = n, p, c
				param = wildcard
			}
		}

		if targetNode == nil {
			break
		}

		params[param] = key

		return targetNode, params, maxUrlLength - len(url) + max
	}

	if key == "" {
		return n, params, maxUrlLength
	}

	if subtreeNode != nil {
		return subtreeNode, params, subtreeDepth
	}

	return nil, nil, 0
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

// Mux is a request multiplexer with the same routing behaviour as the standard libraries net/http ServeMux
type Mux struct {
	root               node
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
	m := &Mux{}
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

	node, params, length := m.root.lookup(path, params)

	trailingSlash := strings.HasSuffix(path, "/")

	var handler http.Handler

	if node != nil {
		if length < len(path) {
			handler = node.subtreeHandler
		} else if trailingSlash {
			if node.subtreeHandler != nil {
				handler = node.subtreeHandler
			} else if m.matchTrailingSlash && node.fixedHandler != nil {
				handler = node.fixedHandler
			}
		} else if node.fixedHandler != nil {
			handler = node.fixedHandler
		} else if node.subtreeHandler != nil {
			handler = redirectToSubdirHandler
		}
	}

	if handler == nil {
		if m.notFoundHandler != nil {
			handler = m.notFoundHandler
		} else {
			handler = defaultNotFoundHandler
		}
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

	if node, remainder := m.root.traverse(pattern); remainder == "/" {
		node.subtreeHandler = handler
	} else {
		node.fixedHandler = handler
	}
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
	if len(pattern) > 1 && pattern[0] == '/' {
		pattern = pattern[1:]
	}

	var i int
	for i < len(pattern) && pattern[i] != '/' {
		i++
	}

	return pattern[:i], pattern[i:]
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
