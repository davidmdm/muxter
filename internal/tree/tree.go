package tree

import (
	"errors"
	"fmt"
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

var errMultipleRegistrations = errors.New("multiple registrations")

type Node[T any] struct {
	Key      string
	Value    *T
	Children []*Node[T]
	Wildcard *Node[T]
	Type     int
}

func (node *Node[T]) Insert(key string, value *T) error {
	colonIndex := strings.IndexByte(key, ':')
	if colonIndex == -1 {
		_, err := node.insert(key, value)
		return err
	}

	pre := key[:colonIndex]

	node, err := node.insert(pre, nil)
	if err != nil {
		return err
	}

	post := key[colonIndex:]

	slashIdx := strings.IndexByte(post, '/')
	if slashIdx == -1 {
		_, err := node.insert(post, value)
		return err
	}

	node, err = node.insert(post[:slashIdx], nil)
	if err != nil {
		return err
	}

	return node.Insert(post[slashIdx:], value)
}

func (node *Node[T]) insert(key string, value *T) (*Node[T], error) {
	if key[0] == ':' {
		if node.Wildcard != nil {
			if node.Wildcard.Key != key[1:] {
				return nil, fmt.Errorf("mismatched wild cards :%s and %s", node.Wildcard.Key, key)
			}
			if value != nil {
				if node.Wildcard.Value != nil {
					return nil, errMultipleRegistrations
				}
				node.Wildcard.Value = value
			}
			return node.Wildcard, nil
		}

		node.Wildcard = &Node[T]{
			Key:      key[1:],
			Value:    value,
			Children: []*Node[T]{},
			Wildcard: nil,
			Type:     Wildcard,
		}
		return node.Wildcard, nil
	}

	for i, n := range node.Children {
		if key == n.Key {
			if value != nil {
				if n.Value != nil {
					return nil, errMultipleRegistrations
				}
				n.Value = value
			}
			return n, nil
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
			return node.Children[i], nil
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

		return targetNode, nil
	}

	targetNode := &Node[T]{
		Key:      key,
		Children: []*Node[T]{},
		Value:    value,
	}

	node.Children = append(node.Children, targetNode)

	return targetNode, nil
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
				continue Walk
			}
			if n.Value != nil && strings.HasPrefix(path+"/", n.Key) {
				return makeRedirectionNode[T]()
			}
			return fallback
		}

		if matchTrailingSlash && path == "/" && node.Value != nil {
			fallback = node
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
