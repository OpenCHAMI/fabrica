//go:build ignore

// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT
// This file contains user-customizable reconciliation logic for ProfileBinding.
//
// ⚠️ This file is safe to edit - it will NOT be overwritten by code generation.
package reconcilers

import (
	"context"
	"fmt"
	"time"

	"github.com/openchami/fabrica/pkg/resource"
	nodev1 "github.com/openchami/node-service/apis/example.fabrica.dev/v1"
)

// reconcileProfileBinding applies profile intent to matched nodes.
func (r *ProfileBindingReconciler) reconcileProfileBinding(ctx context.Context, binding *nodev1.ProfileBinding) error {
	resolved, err := resolveBindingTargets(ctx, r, binding)
	if err != nil {
		return err
	}

	for _, xname := range resolved {
		node, err := findNodeByXname(ctx, r, xname)
		if err != nil {
			return err
		}
		if node == nil {
			continue
		}

		node.Status.EffectiveProfile = binding.Spec.Profile
		node.Status.EffectiveBootProfile = binding.Spec.BootProfile
		node.Status.EffectiveConfigGroups = binding.Spec.ConfigGroups
		node.Status.ResolvedBy = "profilebinding"
		node.Status.ObservedAt = time.Now().UTC()

		if err := r.Client.Update(ctx, node); err != nil {
			return fmt.Errorf("failed to update node %s: %w", node.Spec.Xname, err)
		}
	}

	binding.Status.ResolvedXnames = resolved
	binding.Status.AppliedAt = time.Now().UTC()
	resource.SetCondition(&binding.Status.Conditions, "Applied", "True", "Success", "Profile binding applied")

	return nil
}

func resolveBindingTargets(ctx context.Context, r *ProfileBindingReconciler, binding *nodev1.ProfileBinding) ([]string, error) {
	if binding.Spec.Target.NodeSetUID != "" {
		set, err := r.Client.Get(ctx, "NodeSet", binding.Spec.Target.NodeSetUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get nodeset: %w", err)
		}
		if typed, ok := set.(*nodev1.NodeSet); ok {
			return typed.Status.ResolvedXnames, nil
		}
	}

	if len(binding.Spec.Target.NodeUIDs) > 0 {
		var resolved []string
		for _, uid := range binding.Spec.Target.NodeUIDs {
			item, err := r.Client.Get(ctx, "Node", uid)
			if err != nil {
				return nil, fmt.Errorf("failed to get node %s: %w", uid, err)
			}
			node, ok := item.(*nodev1.Node)
			if ok {
				resolved = append(resolved, node.Spec.Xname)
			}
		}
		return uniqueStrings(resolved), nil
	}

	if len(binding.Spec.Target.Xnames) > 0 {
		return uniqueStrings(binding.Spec.Target.Xnames), nil
	}

	if binding.Spec.Target.Selector != nil {
		return resolveBySelector(ctx, r, *binding.Spec.Target.Selector)
	}

	return nil, fmt.Errorf("no binding target specified")
}

func resolveBySelector(ctx context.Context, r *ProfileBindingReconciler, selector nodev1.NodeSelector) ([]string, error) {
	items, err := r.Client.List(ctx, "Node")
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	var resolved []string
	for _, item := range items {
		node, ok := item.(*nodev1.Node)
		if !ok {
			continue
		}
		if selectorMatchesNode(node, selector) {
			resolved = append(resolved, node.Spec.Xname)
		}
	}
	return applySelectorLimits(uniqueStrings(resolved), selector), nil
}

func findNodeByXname(ctx context.Context, r *ProfileBindingReconciler, xname string) (*nodev1.Node, error) {
	items, err := r.Client.List(ctx, "Node")
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	for _, item := range items {
		node, ok := item.(*nodev1.Node)
		if !ok {
			continue
		}
		if node.Spec.Xname == xname {
			return node, nil
		}
	}
	return nil, nil
}
