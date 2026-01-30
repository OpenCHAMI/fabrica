//go:build ignore

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT
// This file contains user-customizable reconciliation logic for NodeSet.
//
// ⚠️ This file is safe to edit - it will NOT be overwritten by code generation.
package reconcilers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/openchami/fabrica/pkg/resource"
	nodev1 "github.com/openchami/node-service/apis/example.fabrica.dev/v1"
)

// reconcileNodeSet resolves selectors into concrete node xnames.
func (r *NodeSetReconciler) reconcileNodeSet(ctx context.Context, set *nodev1.NodeSet) error {
	nodes, err := r.Client.List(ctx, "Node")
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	var resolved []string
	for _, item := range nodes {
		node, ok := item.(*nodev1.Node)
		if !ok {
			continue
		}
		if selectorMatchesNode(node, set.Spec.Selector) {
			resolved = append(resolved, node.Spec.Xname)
		}
	}

	resolved = uniqueStrings(resolved)
	sort.Strings(resolved)
	resolved = applySelectorLimits(resolved, set.Spec.Selector)

	set.Status.ResolvedXnames = resolved
	set.Status.ObservedAt = time.Now().UTC()
	resource.SetCondition(&set.Status.Conditions, "Resolved", "True", "Success", "NodeSet resolved")

	return nil
}

func selectorMatchesNode(node *nodev1.Node, selector nodev1.NodeSelector) bool {
	if len(selector.Xnames) > 0 && !containsString(selector.Xnames, node.Spec.Xname) {
		return false
	}

	if len(selector.Labels) > 0 && !labelsMatch(selector.Labels, node.Spec.Labels) {
		return false
	}

	if len(selector.Partitions) > 0 && !anyString(selector.Partitions, node.Spec.InventoryGroups) {
		return false
	}

	return true
}

func applySelectorLimits(items []string, selector nodev1.NodeSelector) []string {
	if selector.Count > 0 && selector.Count < len(items) {
		return items[:selector.Count]
	}
	if selector.Percent > 0 && selector.Percent < 100 {
		count := int(float64(len(items)) * (float64(selector.Percent) / 100.0))
		if count < 1 {
			count = 1
		}
		if count < len(items) {
			return items[:count]
		}
	}
	return items
}

func labelsMatch(selector, labels map[string]string) bool {
	for key, val := range selector {
		if labels == nil {
			return false
		}
		if labels[key] != val {
			return false
		}
	}
	return true
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func anyString(first, second []string) bool {
	set := make(map[string]struct{}, len(first))
	for _, item := range first {
		set[strings.TrimSpace(item)] = struct{}{}
	}
	for _, item := range second {
		if _, ok := set[strings.TrimSpace(item)]; ok {
			return true
		}
	}
	return false
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}
