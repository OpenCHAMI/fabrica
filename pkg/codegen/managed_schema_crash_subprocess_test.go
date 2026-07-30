// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagedSchemaSubprocess_process_death_releases_kernel_lock(t *testing.T) {
	// Given
	dir := t.TempDir()
	current := filepath.Join(dir, "schema")
	ready := filepath.Join(dir, "holder.ready")
	holder := startSchemaHelper(t, schemaHelperConfig{
		current: current, ready: ready, release: filepath.Join(dir, "never-release"),
		result: filepath.Join(dir, "holder.result"), identity: "CrashHolder", mode: "crash-lock",
	})
	waitForTestPath(t, ready)
	if err := holder.command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	waitSchemaHelperCrash(t, holder)
	release := filepath.Join(dir, "successor.release")
	if err := os.WriteFile(release, []byte("release"), 0o600); err != nil {
		t.Fatal(err)
	}
	successor := startSchemaHelper(t, schemaHelperConfig{
		current: current, ready: filepath.Join(dir, "successor.ready"), release: release,
		result: filepath.Join(dir, "successor.result"), identity: "AfterCrash",
	})

	// When
	waitSchemaHelper(t, successor)

	// Then
	assertSchemaHelperOwner(t, successor)
	assertCoherentSubprocessSchema(t, current)
	assertPersistentRegularLockFile(t, current)
	assertNoTransactionArtifacts(t, current)
}

func TestManagedSchemaSubprocess_swap_boundary_crash_recovers_on_next_generation(t *testing.T) {
	for _, boundary := range []string{"first", "second"} {
		t.Run(boundary, func(t *testing.T) {
			// Given
			dir := t.TempDir()
			current := filepath.Join(dir, "schema")
			writeManagedSchemaFixture(t, filepath.Join(current, "resource.go"), "type Before struct{}")
			release := filepath.Join(dir, "release")
			if err := os.WriteFile(release, []byte("release"), 0o600); err != nil {
				t.Fatal(err)
			}
			crasher := startSchemaHelper(t, schemaHelperConfig{
				current: current, ready: filepath.Join(dir, "crash.ready"), release: release,
				result: filepath.Join(dir, "crash.result"), identity: "Crash" + boundary, boundary: boundary,
			})
			waitSchemaHelperCrash(t, crasher)
			successor := startSchemaHelper(t, schemaHelperConfig{
				current: current, ready: filepath.Join(dir, "successor.ready"), release: release,
				result: filepath.Join(dir, "successor.result"), identity: "Recovered" + boundary,
			})

			// When
			waitSchemaHelper(t, successor)

			// Then
			assertSchemaHelperOwner(t, successor)
			assertCoherentSubprocessSchema(t, current)
			assertPersistentRegularLockFile(t, current)
			assertNoTransactionArtifacts(t, current)
		})
	}
}

func waitSchemaHelperCrash(t *testing.T, helper *schemaHelperCommand) {
	t.Helper()
	if err := helper.command.Wait(); err == nil {
		t.Fatalf("helper unexpectedly exited cleanly\n%s", helper.output.String())
	}
}

func assertPersistentRegularLockFile(t *testing.T, current string) {
	t.Helper()
	lockPath := current + ".fabrica.lock"
	info, err := os.Lstat(lockPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() != 0 {
		t.Fatalf("persistent lock file = %#v, %v", info, err)
	}
}
