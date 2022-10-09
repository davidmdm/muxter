package main

import (
	"fmt"
)

type Node struct {
	Key      string
	Children []*Node
	Value    *string
}

func (node *Node) Insert(key string, value string) {
	if key == "" {
		return
	}

	for i, n := range node.Children {
		if key == n.Key {
			return
		}

		cp := commonPrefixIndex(n.Key, key)
		if cp == 0 {
			continue
		}

		if cp == len(n.Key) {
			n.Insert(key[cp:], value)
			return
		}

		n.Key = n.Key[cp:]

		if cp == len(key) {
			node.Children[i] = &Node{
				Key:      key,
				Children: []*Node{n},
				Value:    &value,
			}
			return
		}

		node.Children[i] = &Node{
			Key: key[:cp],
			Children: []*Node{
				n,
				{
					Key:      key[cp:],
					Children: []*Node{},
					Value:    &value,
				},
			},
		}

		return
	}

	node.Children = append(node.Children, &Node{
		Key:      key,
		Children: []*Node{},
		Value:    &value,
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func commonPrefixIndex(a, b string) (i int) {
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

func main() {
	root := NewRootTree()

	fmt.Println("---------------------")

	root.Insert("/", "root")
	print(root, "", "  ")

	fmt.Println("---------------------")

	root.Insert("/api/classes", "get all classes")
	print(root, "", "  ")

	fmt.Println("---------------------")

	root.Insert("/api/classes", "copy")

	fmt.Println("---------------------")

	root.Insert("/api/class", "get single class")
	print(root, "", "  ")

	fmt.Println("---------------------")

	root.Insert("/api/book", "get a book")
	print(root, "", "  ")

	fmt.Println("---------------------")

	root.Insert("/public/", "public root")
	print(root, "", "  ")

	fmt.Println("---------------------")

	root.Insert("/app/", "app root")
	print(root, "", "  ")
	fmt.Println("---------------------")
	root.Insert("jesus", "what would jesus do?")
	print(root, "", "  ")
}

func print(n *Node, prefix, indent string) {
	value := "<nil>"
	if n.Value != nil {
		value = *n.Value
	}
	fmt.Printf("%s%q (%s)\n", prefix, n.Key, value)
	for _, c := range n.Children {
		print(c, prefix+indent, indent)
	}
}
