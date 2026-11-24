// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Fixture modules use the repository minimum even when a test changes directory.
func repositoryGoVersion(t testing.TB) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate repository go.mod")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(source), "..", "..", "go.mod"))
	if err != nil {
		t.Fatalf("read repository go.mod: %v", err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "go" {
			return fields[1]
		}
	}
	t.Fatal("repository go.mod has no go directive")
	return ""
}

func fixtureGoCommand(args ...string) *exec.Cmd {
	// These tests build and execute on the same host. Using their toolchain is
	// deliberate: PATH may still select an older Go before toolchain switching.
	goRoot := runtime.GOROOT() //nolint:staticcheck // Same-host test fixtures need the executing toolchain, not the system default.
	cmd := exec.Command(filepath.Join(goRoot, "bin", "go"), args...)
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off")
	return cmd
}

func TestFixtureGoCommandKeepsTestToolchain(t *testing.T) {
	// A caller's PATH, workspace or explicit older toolchain must not redirect
	// a fixture to a compiler other than the one running this test binary.
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GOTOOLCHAIN", "go1.0.0")
	t.Setenv("GOWORK", filepath.Join(t.TempDir(), "missing.work"))
	cmd := fixtureGoCommand("env", "GOVERSION", "GOTOOLCHAIN", "GOWORK")
	cmd.Dir = t.TempDir()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture Go command: %v\n%s", err, output)
	}
	want := runtime.Version() + "\nlocal\noff\n"
	if string(output) != want {
		t.Fatalf("fixture toolchain = %q, want %q", output, want)
	}
}
