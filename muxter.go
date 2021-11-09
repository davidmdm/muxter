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

func (n *node) lookup(url string) (handler http.Handler, params map[string]string, deepness int) {
	var key string
	var subtreeHandler http.Handler
	var subtreeDeepness int

	orignalUrlLength := len(url)

	for {
		if n.subtreeHandler != nil {
			subtreeHandler = n.subtreeHandler
			subtreeDeepness = orignalUrlLength - len(url)
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

				return handler, params, orignalUrlLength - len(url) + max
			}

			if subtreeHandler != nil {
				return subtreeHandler, params, subtreeDeepness
			}

			return defaultNotFoundHandler, params, 0
		}

		if url == "" && n.fixedHandler != nil {
			return n.fixedHandler, params, orignalUrlLength
		}

		if subtreeHandler != nil {
			return subtreeHandler, params, deepness
		}
		return defaultNotFoundHandler, params, 0
	}
}

type Mux struct {
	get, post, patch, put, delete node
}

func New() *Mux {
	return &Mux{}
}

func (m *Mux) Get(pattern string, handler http.Handler) {
	m.add("get", pattern, handler)
}

func (m *Mux) Post(pattern string, handler http.Handler) {
	m.add("post", pattern, handler)
}

func (m *Mux) Patch(pattern string, handler http.Handler) {
	m.add("patch", pattern, handler)
}

func (m *Mux) Put(pattern string, handler http.Handler) {
	m.add("put", pattern, handler)
}

func (m *Mux) Delete(pattern string, handler http.Handler) {
	m.add("delete", pattern, handler)
}

func (m *Mux) add(method, pattern string, fn http.Handler) {
	var n *node
	switch method {
	case "get":
		n = &m.get
	case "post":
		n = &m.post
	case "patch":
		n = &m.patch
	case "put":
		n = &m.put
	case "delete":
		n = &m.delete
	default:
		panic("muxter: mux.add: invalid method: " + method)
	}

	var key string
	pattern = cleanPath(pattern)

	for {
		key, pattern = split(pattern)
		if key != "" {
			if key[0] == ':' {
				if n.wildcards == nil {
					n.wildcards = make(map[string]*node)
				}
				next, ok := n.wildcards[key[1:]]
				if !ok {
					next = &node{}
					n.wildcards[key[1:]] = next
				}
				n = next
			} else {
				if n.children == nil {
					n.children = make(map[string]*node)
				}
				next, ok := n.children[key]
				if !ok {
					next = &node{}
					n.children[key] = next
				}
				n = next
			}
			continue
		}

		if pattern == "/" {
			n.subtreeHandler = fn
		} else {
			n.fixedHandler = fn
		}

		break
	}
}

func (m Mux) lookupHandler(method, url string) (http.Handler, map[string]string) {
	var n node
	switch strings.ToLower(method) {
	case "get":
		n = m.get
	case "post":
		n = m.post
	case "patch":
		n = m.patch
	case "put":
		n = m.put
	case "delete":
		n = m.delete
	default:
		panic("muxter: mux.lookupHandler: invalid method: " + method)
	}

	handler, params, _ := n.lookup(url)
	return handler, params
}

type paramKeyType int

var paramKey paramKeyType

func (m Mux) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	handler, params := m.lookupHandler(req.Method, cleanPath(req.URL.Path))

	if params != nil {
		ctx := context.WithValue(req.Context(), paramKey, params)
		req = req.WithContext(ctx)
	}

	handler.ServeHTTP(res, req)
}

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
