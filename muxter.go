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

type node struct {
	wildcards      map[string]*node
	children       map[string]*node
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
			if n.children != nil {
				if next, ok := n.children[key]; ok {
					n = next
					continue
				}
			}

			var handler http.Handler
			var card string
			max := -1
			for wildcard, wildNode := range n.wildcards {
				h, p, c := wildNode.lookup(url)
				if c > max {
					handler, params, max = h, p, c
					card = wildcard
				}
			}

			if handler != nil {
				if params == nil {
					params = make(map[string]string)
				}
				params[card] = key

				return handler, params, maxUrlLength - len(url) + max
			}

			if subtreeHandler != nil {
				return subtreeHandler, params, subtreeDepth
			}

			return defaultNotFoundHandler, params, 0
		}

		if url == "" && n.fixedHandler != nil {
			return n.fixedHandler, params, maxUrlLength
		}

		if subtreeHandler != nil {
			return subtreeHandler, params, subtreeDepth
		}
		return defaultNotFoundHandler, params, 0
	}
}

type Mux struct {
	root node
}

func New() *Mux {
	return &Mux{}
}

func (m Mux) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	handler, params, _ := m.root.lookup(cleanPath(req.URL.Path))

	if params != nil {
		ctx := context.WithValue(req.Context(), paramKey, params)
		req = req.WithContext(ctx)
	}

	handler.ServeHTTP(res, req)
}

func (m *Mux) HandleFunc(pattern string, handler http.HandlerFunc) {
	m.Handle(pattern, handler)
}

func (m *Mux) Handle(pattern string, handler http.Handler) {
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
				if n.children == nil {
					n.children = make(map[string]*node)
				}
				nodeMap = n.children
			}

			next, ok := nodeMap[key]
			if !ok {
				next = &node{}
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
