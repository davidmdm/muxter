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

var errMultipleRegistrations = errors.New("multiple registrations")

type Node[T any] struct {
	Key      string
	Value    *T
	Children []*Node[T]
	Indices  []byte
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
				Indices:  []byte{n.Key[0]},
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
			Indices:  []byte{n.Key[0], targetNode.Key[0]},
		}

		return targetNode, nil
	}

	targetNode := &Node[T]{
		Key:      key,
		Children: []*Node[T]{},
		Value:    value,
	}

	node.Children = append(node.Children, targetNode)
	node.Indices = append(node.Indices, targetNode.Key[0])

	return targetNode, nil
}

func (node *Node[T]) Lookup(path string, params map[string]string, matchTrailingSlash bool) (target *Node[T]) {
	var fallback *Node[T]
	defer func() {
		if target == nil || (target.Value == nil && target.Type != Redirect) {
			target = fallback
		}
	}()

	var wildcardbackup *Node[T]

Walk:
	for {
		if node.Type == Wildcard {
			if idx := strings.IndexByte(path, '/'); idx == -1 {
				params[node.Key] = path
				return node
			} else {
				params[node.Key] = path[:idx]
				path = path[idx:]
			}
		} else {
			if !strings.HasPrefix(path, node.Key) {
				if wildcardbackup != nil {
					node = wildcardbackup
					continue Walk
				}
				if node.Value != nil && path+"/" == node.Key {
					return &Node[T]{Type: Redirect}
				}
				return nil
			}
			path = strings.TrimPrefix(path, node.Key)
			if path == "" {
				return node
			}
			if node.IsSubdirNode() {
				fallback = node
			}
		}

		if matchTrailingSlash && path == "/" && node.Value != nil {
			fallback = node
		}

		wildcardbackup = node.Wildcard

		targetIndice := path[0]
		for i, c := range node.Indices {
			if c == targetIndice {
				node = node.Children[i]
				continue Walk
			}
		}

		if node.Wildcard != nil {
			node = node.Wildcard
			continue Walk
		}

		return nil
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
