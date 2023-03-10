package muxter

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/davidmdm/muxter/internal"
)

const (
	static = iota
	wildcard
	expression
	catchall
)

var errMultipleRegistrations = errors.New("multiple registrations")

type value struct {
	handler    Handler
	pattern    string
	isRedirect bool
}

type node struct {
	Value      *value
	Wildcard   *node
	Catchall   *node
	Expression *node
	Key        string
	Children   []*node
	Indices    []byte
	Type       int
	expression *regexp.Regexp
}

func (n *node) Insert(key string, value *value) error {
	idx := strings.IndexAny(key, "#:*")
	if idx == -1 {
		_, err := n.insert(key, value)
		return err
	}

	pre := key[:idx]

	n, err := n.insert(pre, nil)
	if err != nil {
		return err
	}

	post := key[idx:]
	slashIdx := func() int {
		if post[0] != '#' {
			return strings.IndexByte(post, '/')
		}
		i := regexp.MustCompile(`[^\\]/`).FindStringIndex(post)
		if i == nil {
			return -1
		}
		return i[1] - 1
	}()

	if slashIdx == -1 {
		_, err := n.insert(post, value)
		return err
	}

	if post[0] == '*' {
		return fmt.Errorf("cannot register segments after a catchall expression %q", post[:slashIdx])
	}

	n, err = n.insert(post[:slashIdx], nil)
	if err != nil {
		return err
	}

	return n.Insert(post[slashIdx:], value)
}

func (n *node) insert(key string, value *value) (*node, error) {
	switch key[0] {
	case '#':
		idx := strings.IndexByte(key, ':')
		if idx == -1 {
			return nil, fmt.Errorf("invalid regexp param: %s", key)
		}

		k := key[1:idx]

		exp, err := regexp.Compile(fmt.Sprintf("^(%s)", key[idx+1:]))
		if err != nil {
			return nil, err
		}

		if n.Expression != nil {
			oldCanonicalKey := fmt.Sprintf("#%s:%s", n.Expression.Key, n.expression)
			newCanonicalKey := fmt.Sprintf("#%s:%s", k, exp)

			if oldCanonicalKey != newCanonicalKey {
				return nil, fmt.Errorf("mismatched regexp segments %s  and %s", oldCanonicalKey, newCanonicalKey)
			}
			if value != nil {
				if n.Expression.Value != nil {
					return nil, errMultipleRegistrations
				}
				n.Expression.Value = value
			}
			return n.Expression, nil
		}

		n.Expression = &node{
			Value:      value,
			Key:        k,
			Type:       expression,
			expression: exp,
		}

		return n.Expression, nil

	case ':':
		if n.Wildcard != nil {
			if n.Wildcard.Key != key[1:] {
				return nil, fmt.Errorf("mismatched wild cards :%s and %s", n.Wildcard.Key, key)
			}
			if value != nil {
				if n.Wildcard.Value != nil {
					return nil, errMultipleRegistrations
				}
				n.Wildcard.Value = value
			}
			return n.Wildcard, nil
		}

		n.Wildcard = &node{
			Key:   key[1:],
			Value: value,
			Type:  wildcard,
		}

		return n.Wildcard, nil

	case '*':
		if n.Catchall != nil {
			if n.Catchall.Key != key[1:] {
				return nil, fmt.Errorf("mismatched wild cards *%s and %s", n.Catchall.Key, key)
			}
			return nil, errMultipleRegistrations
		}
		n.Catchall = &node{
			Key:   key[1:],
			Value: value,
			Type:  catchall,
		}
		return n.Catchall, nil
	}

	for i, childNode := range n.Children {
		if key == childNode.Key {
			if value != nil {
				if childNode.Value != nil {
					return nil, errMultipleRegistrations
				}
				childNode.Value = value
			}
			return childNode, nil
		}

		cp := commonPrefixLength(childNode.Key, key)
		if cp == 0 {
			continue
		}

		if cp == len(childNode.Key) {
			return childNode.insert(key[cp:], value)
		}

		childNode.Key = childNode.Key[cp:]

		if cp == len(key) {
			n.Children[i] = &node{
				Key:      key,
				Children: []*node{childNode},
				Indices:  []byte{childNode.Key[0]},
				Value:    value,
			}
			return n.Children[i], nil
		}

		targetNode := &node{
			Key:      key[cp:],
			Children: []*node{},
			Value:    value,
		}

		n.Children[i] = &node{
			Key:      key[:cp],
			Children: []*node{childNode, targetNode},
			Indices:  []byte{childNode.Key[0], targetNode.Key[0]},
		}

		return targetNode, nil
	}

	targetNode := &node{
		Key:      key,
		Children: []*node{},
		Value:    value,
	}

	n.Children = append(n.Children, targetNode)
	n.Indices = append(n.Indices, targetNode.Key[0])

	return targetNode, nil
}

func (n *node) Lookup(path string, params *[]internal.Param, matchTrailingSlash bool) (result *value) {
	var fallback *value
	defer func() {
		if result == nil {
			result = fallback
		}
	}()

	var wildcardbackup *node

Walk:
	for {
		switch n.Type {
		case static:
			if !strings.HasPrefix(path, n.Key) {
				if wildcardbackup != nil {
					n = wildcardbackup
					continue Walk
				}
				if n.Value != nil && path+"/" == n.Key {
					return &value{isRedirect: true, pattern: n.Value.pattern[:len(n.Value.pattern)-1]}
				}
				return nil
			}
			path = path[len(n.Key):]
			if path == "" {
				return n.Value
			}
			if n.IsSubdirNode() {
				fallback = n.Value
			}
		case wildcard:
			if idx := strings.IndexByte(path, '/'); idx == -1 {
				*params = append(*params, internal.Param{
					Key:   n.Key,
					Value: path,
				})
				return n.Value
			} else {
				*params = append(*params, internal.Param{
					Key:   n.Key,
					Value: path[:idx],
				})
				path = path[idx:]
			}
		case catchall:
			*params = append(*params, internal.Param{
				Key:   n.Key,
				Value: path,
			})
			return n.Value
		case expression:
			i := n.expression.FindStringIndex(path)
			if i == nil {
				return nil
			}
			*params = append(*params, internal.Param{
				Key:   n.Key,
				Value: path[:i[1]],
			})
			path = path[i[1]:]
		}

		if matchTrailingSlash && path == "/" && n.Value != nil {
			fallback = n.Value
		}

		wildcardbackup = n.Wildcard

		targetIndice := path[0]
		for i, c := range n.Indices {
			if c == targetIndice {
				n = n.Children[i]
				continue Walk
			}
		}

		if n.Catchall != nil {
			n = n.Catchall
			continue Walk
		}

		if n.Wildcard != nil {
			n = n.Wildcard
			continue Walk
		}
		if n.Expression != nil {
			n = n.Expression
			continue Walk
		}

		return nil
	}
}

func (node *node) IsSubdirNode() bool {
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
