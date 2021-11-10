package muxter

import (
	"context"
	"net/http"
	"path"
	"strings"
)

var _ http.Handler = Mux{}

var defaultNotFoundHandler http.HandlerFunc = func(rw http.ResponseWriter, r *http.Request) {
	http.Error(rw, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

var redirectToSubdirHandler http.HandlerFunc = func(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Location", r.URL.Path+"/")
	rw.WriteHeader(http.StatusMovedPermanently)
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
			if n.segments != nil {
				if next, ok := n.segments[key]; ok {
					n = next
					continue
				}
			}

			var handler http.Handler
			var param string
			max := -1

			for wildcard, wildNode := range n.wildcards {
				h, p, c := wildNode.lookup(url)
				if c > max {
					handler, params, max = h, p, c
					param = wildcard
				}
			}

			if handler != nil {
				if params == nil {
					params = make(map[string]string)
				}
				params[param] = key

				return handler, params, maxUrlLength - len(url) + max
			}

			if subtreeHandler != nil {
				return subtreeHandler, params, subtreeDepth
			}

			return defaultNotFoundHandler, params, 0
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

		return defaultNotFoundHandler, params, 0
	}
}

// Mux is a request multiplexer with the same routing behaviour as the standard libraries net/http ServeMux
type Mux struct {
	root node
}

// New returns a pointer to a new muxter.Mux
func New() *Mux {
	return &Mux{}
}

// ServeHTTP implements the net/http Handler interface.
func (m Mux) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	handler, params, _ := m.root.lookup(cleanPath(req.URL.Path))

	if params != nil {
		ctx := context.WithValue(req.Context(), paramKey, params)
		req = req.WithContext(ctx)
	}

	handler.ServeHTTP(res, req)
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
	handler = withMiddleware(handler, middlewares...)

	n := &m.root

	var key string
	pattern = cleanPath(pattern)

	for {
		key, pattern = split(pattern)
		if key != "" {
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

			continue
		}

		if pattern == "/" {
			n.subtreeHandler = handler
		} else {
			n.fixedHandler = handler
		}

		return
	}
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
