// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"os"

	v1 "example.com/test/apis/v1"
	fabricaStorage "github.com/openchami/fabrica/pkg/storage"
)

func SeedNode(node v1.Node) {
	directory, err := os.MkdirTemp(".", "conditional-nodes-")
	if err != nil {
		panic(err)
	}
	backend, err := fabricaStorage.NewFileBackend(directory)
	if err != nil {
		panic(err)
	}
	Init(backend)
	if err := SaveNode(context.Background(), &node); err != nil {
		panic(err)
	}
}
func NodeForTest(uid string) v1.Node {
	node, err := LoadNode(context.Background(), uid)
	if err != nil {
		return v1.Node{}
	}
	return *node
}
