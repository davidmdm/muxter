package main

import (
	"fmt"
	"strings"
)

const (
	Static   = 0
	Wildcard = 1
)

type Node struct {
	Key      string
	Value    *string
	Children []*Node
	Wildcard *Node
	Type     int
}

func (n *Node) String() string {
	if n == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%+v", *n)
}

func (node *Node) Insert(key string, value *string) {
	colonIndex := strings.IndexByte(key, ':')
	if colonIndex == -1 {
		node.insert(key, value)
		return
	}

	pre := key[:colonIndex]
	node = node.insert(pre, nil)

	post := key[colonIndex:]

	slashIdx := strings.IndexByte(post, '/')
	if slashIdx == -1 || len(post) == slashIdx+1 {
		node.insert(post, value)
		return
	}

	node = node.insert(post[:slashIdx], nil)
	node.Insert(post[slashIdx:], value)
}

func (node *Node) insert(key string, value *string) *Node {
	if key == "" {
		return node
	}

	if key[0] == ':' {
		node.Wildcard = &Node{
			Key:      key[1:],
			Value:    value,
			Children: []*Node{},
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
			node.Children[i] = &Node{
				Key:      key,
				Children: []*Node{n},
				Value:    value,
			}
			return node.Children[i]
		}

		targetNode := &Node{
			Key:      key[cp:],
			Children: []*Node{},
			Value:    value,
		}

		node.Children[i] = &Node{
			Key:      key[:cp],
			Children: []*Node{n, targetNode},
		}

		return targetNode
	}

	targetNode := &Node{
		Key:      key,
		Children: []*Node{},
		Value:    value,
	}

	node.Children = append(node.Children, targetNode)

	return targetNode
}

func (node *Node) Lookup(path string, params map[string]string) (*Node, map[string]string) {
	var subdirNode *Node

Walk:
	for {
		if node.IsSubdirNode() {
			subdirNode = node
		}
		if path == "" {
			return subdirNode, params
		}
		if path == node.Key {
			return node, params
		}

		if node.Type == Wildcard {
			slashIdx := strings.IndexByte(path, '/')
			if slashIdx == -1 {
				params[node.Key] = path
				return node, params
			}
			params[node.Key] = path[:slashIdx]
			path = path[slashIdx:]
		}

		for _, n := range node.Children {
			if path == n.Key {
				return n, params
			}
			i := commonPrefixLength(path, n.Key)
			if i == 0 {
				continue
			}

			node, path = n, path[1:]
			continue Walk
		}

		if node.Wildcard != nil {
			node = node.Wildcard
			continue Walk
		}

		return subdirNode, params
	}
}

func (node *Node) IsSubdirNode() bool {
	return node != nil && strings.HasSuffix(node.Key, "/")
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

func NewRootTree() *Node {
	return &Node{
		Key:      "",
		Children: []*Node{},
	}
}

// func main() {
// 	root := NewRootTree()

// 	root.Insert("/public/index.js", ptr("index js"))
// 	root.Insert("/public/index.html", ptr("my index html"))
// 	root.Insert("/context/:ctx/policy/:id", ptr("My PTR"))

// 	root.Insert("/public/", ptr("root public directory"))

// 	p := map[string]string{}

// 	node, params := root.Lookup("/public/somefile.txt", p)
// 	fmt.Println(node, params)

// 	// fmt.Println("---------------------")

// 	// root.Insert("/", ptr("root"))
// 	// print(root, "", "  ", false)

// 	// fmt.Println("---------------------")

// 	// root.Insert("/api/classes", ptr("get all classes"))
// 	// print(root, "", "  ", false)

// 	// fmt.Println("---------------------")

// 	// root.Insert("/api/classes/:id", ptr("copy"))
// 	// print(root, "", "  ", false)

// 	// fmt.Println("---------------------")

// 	// root.Insert("/api/class", ptr("get single class"))
// 	// print(root, "", "  ", false)

// 	// fmt.Println("---------------------")

// 	// root.Insert("/api/book", ptr("get a book"))
// 	// print(root, "", "  ", false)

// 	// fmt.Println("---------------------")

// 	// root.Insert("/public/", ptr("public root"))
// 	// print(root, "", "  ", false)

// 	// fmt.Println("---------------------")

// 	// root.Insert("/app/", ptr("app root"))
// 	// print(root, "", "  ", false)

// 	// fmt.Println("---------------------")

// 	// root.Insert("jesus", ptr("what would jesus do?"))

// 	// print(root, "", "  ", false)
// }

// func print(n *Node, prefix, indent string, wild bool) {
// 	if n == nil {
// 		return
// 	}

// 	value := "<nil>"
// 	if n.Value != nil {
// 		value = *n.Value
// 	}

// 	if wild {
// 		fmt.Printf("%s%q *%s*\n", prefix, n.Key, value)
// 	} else {
// 		fmt.Printf("%s%q (%s)\n", prefix, n.Key, value)
// 	}

// 	for _, c := range n.Children {
// 		print(c, prefix+indent, indent, false)
// 	}

// 	print(n.Wildcard, prefix+indent, indent, true)
// }

// func ptr[T any](value T) *T { return &value }

/*

/api/owner/spec/:dyn/something
/api/owner/:oid/else

input -> /api/owner/spec/else

*/
