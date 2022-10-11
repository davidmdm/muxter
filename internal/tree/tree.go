package tree

import (
	"strings"
)

const (
	Static   = 0
	Wildcard = 1
	Redirect = 2
)

func makeRedirectionNode[T any]() *Node[T] {
	return &Node[T]{
		Type: Redirect,
	}
}

type Node[T any] struct {
	Key      string
	Value    *T
	Children []*Node[T]
	Wildcard *Node[T]
	Type     int
}

func (node *Node[T]) Insert(path string, value *T) {
	if value == nil {
		panic("http handler cannot be nil")
	}

	colonIndex := strings.IndexByte(path, ':')
	if colonIndex == -1 {
		node.insert(path, value)
		return
	}

	pre := path[:colonIndex]
	node = node.insert(pre, nil)

	post := path[colonIndex:]

	slashIdx := strings.IndexByte(post, '/')
	if slashIdx == -1 {
		node.insert(post, value)
		return
	}

	node = node.insert(post[:slashIdx], nil)
	node.Insert(post[slashIdx:], value)
}

func (node *Node[T]) insert(key string, value *T) *Node[T] {
	if key == "" {
		return node
	}

	if key[0] == ':' {
		node.Wildcard = &Node[T]{
			Key:      key[1:],
			Value:    value,
			Children: []*Node[T]{},
			Wildcard: nil,
			Type:     Wildcard,
		}
		return node.Wildcard
	}

	for i, n := range node.Children {
		if key == n.Key {
			return n
		}

		cp := commonPrefixLength(n.Key, key)
		if cp == 0 {
			continue
		}

		if cp == len(n.Key) {
			return n.insert(key[cp:], value)
		}

		n.Key = n.Key[cp:]

		if cp == len(key) {
			node.Children[i] = &Node[T]{
				Key:      key,
				Children: []*Node[T]{n},
				Value:    value,
			}
			return node.Children[i]
		}

		targetNode := &Node[T]{
			Key:      key[cp:],
			Children: []*Node[T]{},
			Value:    value,
		}

		node.Children[i] = &Node[T]{
			Key:      key[:cp],
			Children: []*Node[T]{n, targetNode},
		}

		return targetNode
	}

	targetNode := &Node[T]{
		Key:      key,
		Children: []*Node[T]{},
		Value:    value,
	}

	node.Children = append(node.Children, targetNode)

	return targetNode
}

func (node *Node[T]) Lookup(path string, params map[string]string, matchTrailingSlash bool) *Node[T] {
	var fallback *Node[T]

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
				if matchTrailingSlash && path == "/" && node.Value != nil {
					fallback = node
				}
				continue Walk
			}
			if n.Value != nil && strings.HasPrefix(path+"/", n.Key) {
				return makeRedirectionNode[T]()
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

func (node *Node[T]) IsSubdirNode() bool {
	return node != nil && node.Value != nil && strings.HasSuffix(node.Key, "/")
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
