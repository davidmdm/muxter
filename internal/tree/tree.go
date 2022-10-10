package tree

import (
	"net/http"
	"strings"
)

const (
	Static   = 0
	Wildcard = 1
	Redirect = 2
)

var redirectionNode = &Node{
	Key: "",
	Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", r.URL.Path+"/")
		w.WriteHeader(301)
	}),
	Children: []*Node{},
	Type:     Redirect,
}

type Node struct {
	Key      string
	Handler  http.Handler
	Children []*Node
	Wildcard *Node
	Type     int
}

func (node *Node) Insert(path string, handler http.Handler) {
	if handler == nil {
		panic("http handler cannot be nil")
	}

	colonIndex := strings.IndexByte(path, ':')
	if colonIndex == -1 {
		node.insert(path, handler)
		return
	}

	pre := path[:colonIndex]
	node = node.insert(pre, nil)

	post := path[colonIndex:]

	slashIdx := strings.IndexByte(post, '/')
	if slashIdx == -1 {
		node.insert(post, handler)
		return
	}

	node = node.insert(post[:slashIdx], nil)
	node.Insert(post[slashIdx:], handler)
}

func (node *Node) insert(path string, handler http.Handler) *Node {
	if path == "" {
		return node
	}

	if path[0] == ':' {
		node.Wildcard = &Node{
			Key:      path[1:],
			Handler:  handler,
			Children: []*Node{},
			Wildcard: nil,
			Type:     Wildcard,
		}
		return node.Wildcard
	}

	for i, n := range node.Children {
		if path == n.Key {
			return n
		}

		cp := commonPrefixLength(n.Key, path)
		if cp == 0 {
			continue
		}

		if cp == len(n.Key) {
			return n.insert(path[cp:], handler)
		}

		n.Key = n.Key[cp:]

		if cp == len(path) {
			node.Children[i] = &Node{
				Key:      path,
				Children: []*Node{n},
				Handler:  handler,
			}
			return node.Children[i]
		}

		targetNode := &Node{
			Key:      path[cp:],
			Children: []*Node{},
			Handler:  handler,
		}

		node.Children[i] = &Node{
			Key:      path[:cp],
			Children: []*Node{n, targetNode},
		}

		return targetNode
	}

	targetNode := &Node{
		Key:      path,
		Children: []*Node{},
		Handler:  handler,
	}

	node.Children = append(node.Children, targetNode)

	return targetNode
}

func (node *Node) Lookup(path string, params map[string]string, matchTrailingSlash bool) *Node {
	var fallback *Node

Walk:
	for {
		if node.IsSubdirNode() {
			fallback = node
		}
		if path == "" {
			return fallback
		}
		if path == node.Key && node.Type == Static {
			return node
		}

		if node.Type == Wildcard {
			slashIdx := strings.IndexByte(path, '/')
			if slashIdx == -1 {
				params[node.Key] = path
				return node
			}
			params[node.Key] = path[:slashIdx]
			path = path[slashIdx:]
		}

		for _, n := range node.Children {
			if path[0] != n.Key[0] {
				continue
			}
			if path == n.Key {
				return n
			}
			if strings.HasPrefix(path, n.Key) {
				node, path = n, path[len(n.Key):]
				if matchTrailingSlash && path == "/" && node.Handler != nil {
					fallback = node
				}
				continue Walk
			}
			if n.Handler != nil && strings.HasPrefix(path+"/", n.Key) {
				return redirectionNode
			}
			return fallback
		}

		if node.Wildcard != nil {
			node = node.Wildcard
			continue Walk
		}

		return fallback
	}
}

func (node *Node) IsSubdirNode() bool {
	return node != nil && node.Handler != nil && strings.HasSuffix(node.Key, "/")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func commonPrefixLength(a, b string) (i int) {
	for ; i < min(len(a), len(b)); i++ {
		if a[i] != b[i] {
			break
		}
	}
	return
}