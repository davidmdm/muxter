package main

import (
	"fmt"
)

type Node struct {
	Key      string
	Children []*Node
}

func (node *Node) Insert(key string) {
	if key == "" {
		return
	}

	var matched bool
	for i, n := range node.Children {
		if key == n.Key {
			return
		}

		cp := commonPrefixIndex(n.Key, key)
		if cp == 0 {
			continue
		}
		matched = true

		if cp == len(n.Key) {
			n.Insert(key[cp:])
			return
		}

		prefix := &Node{
			Key:      key[:cp],
			Children: []*Node{},
		}

		if len(n.Key) > cp {
			n.Key = n.Key[cp:]
			prefix.Children = append(prefix.Children, n)
		}
		if len(key) > cp {
			prefix.Children = append(prefix.Children, &Node{
				Key:      key[cp:],
				Children: []*Node{},
			})
		}

		node.Children[i] = prefix

		break
	}

	if matched {
		return
	}

	node.Children = append(node.Children, &Node{
		Key:      key,
		Children: []*Node{},
	})

	print(node, "", "  ")
	fmt.Println("---------------------------------------")
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

	root.Insert("/")
	root.Insert("/api/classes")
	root.Insert("/api/classes")

	root.Insert("/api/class")
	root.Insert("/api/book")
	root.Insert("/public")
	root.Insert("/app")
	root.Insert("jesus")

	print(root, "", "  ")
}

func print(n *Node, prefix, indent string) {
	fmt.Printf("%s%q\n", prefix, n.Key)
	for _, c := range n.Children {
		print(c, prefix+indent, indent)
	}
}
