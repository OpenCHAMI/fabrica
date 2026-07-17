// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package codegen

import "testing"

func TestResourceOperationsFromNames(t *testing.T) {
	ops := ResourceOperationsFromNames([]string{"list", "update-status"})
	if !ops.List || !ops.UpdateStatus {
		t.Fatalf("expected list and update-status operations to be enabled: %#v", ops)
	}
	if ops.Get || ops.Create || ops.Patch || ops.Delete || ops.PatchStatus {
		t.Fatalf("unexpected operations enabled: %#v", ops)
	}
}

func TestResourceOperationsFromNamesDefault(t *testing.T) {
	ops := ResourceOperationsFromNames(nil)
	if !ops.List || !ops.Get || !ops.Create || !ops.Update || !ops.Patch || !ops.Delete || !ops.UpdateStatus || !ops.PatchStatus {
		t.Fatalf("expected default operations to enable full CRUD plus status: %#v", ops)
	}
}

func TestOperationListIncludesOnlyGeneratedCLICommands(t *testing.T) {
	operationList := templateFuncs["operationList"].(func(ResourceOperations) string)
	got := operationList(ResourceOperations{List: true, UpdateStatus: true, PatchStatus: true})
	want := "list"
	if got != want {
		t.Fatalf("operationList() = %q, want %q", got, want)
	}
}

func TestOperationListReportsNoneForStatusOnlyResource(t *testing.T) {
	operationList := templateFuncs["operationList"].(func(ResourceOperations) string)
	got := operationList(ResourceOperations{UpdateStatus: true, PatchStatus: true})
	if got != "none" {
		t.Fatalf("operationList() = %q, want none", got)
	}
}
