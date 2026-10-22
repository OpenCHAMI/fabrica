// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"fmt"

	v1 "example.com/test/apis/v1"
)

var nodes = map[string]v1.Node{}

func SeedNode(node v1.Node) {
	nodes = map[string]v1.Node{node.Metadata.UID: node}
}

func NodeForTest(uid string) v1.Node {
	return nodes[uid]
}

func LoadAllNodes(context.Context) ([]v1.Node, error) {
	result := make([]v1.Node, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, node)
	}
	return result, nil
}

func LoadNode(_ context.Context, uid string) (*v1.Node, error) {
	node, ok := nodes[uid]
	if !ok {
		return nil, fmt.Errorf("node %s not found", uid)
	}
	return &node, nil
}

func SaveNode(_ context.Context, node *v1.Node) error {
	nodes[node.Metadata.UID] = *node
	return nil
}

func DeleteNode(_ context.Context, uid string) error {
	delete(nodes, uid)
	return nil
}
