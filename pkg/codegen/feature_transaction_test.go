// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func transactionSelection(g *Generator, enabled bool) {
	f := g.Config.Features
	f.MetricsEnabled, f.AuthEnabled, f.EventsEnabled, f.ReconciliationEnabled = enabled, enabled, enabled, enabled
	f.ValidationEnabled, f.ConditionalEnabled, f.VersioningEnabled = enabled, enabled, enabled
	f.ValidationMode = "disabled"
	if enabled {
		f.ValidationMode = "strict"
	}
	g.SetFeatures(f)
}

func transactionFixture(t *testing.T, initial bool) *Generator {
	t.Helper()
	g := featureTestGenerator(t)
	g.Version = "v0.4.14"
	g.Commit = "original"
	transactionSelection(g, initial)
	if err := g.GenerateFeatures(); err != nil {
		t.Fatal(err)
	}
	return g
}

func transactionSnapshot(t *testing.T, g *Generator) map[string][]byte {
	t.Helper()
	snapshot := make(map[string][]byte)
	for _, output := range append(g.featureOutputs(), featureOutput{path: filepath.Join(g.OutputDir, featureManifestName)}) {
		data, err := os.ReadFile(output.path)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		snapshot[output.path] = data
	}
	return snapshot
}

func assertTransactionComplete(t *testing.T, g *Generator) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(g.OutputDir, featureManifestName))
	if err != nil {
		t.Fatal(err)
	}
	var manifest featureManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Config != g.Config.Features.Normalize() {
		t.Fatalf("manifest config=%+v, wanted %+v", manifest.Config, g.Config.Features.Normalize())
	}
	count := 0
	for _, output := range g.featureOutputs() {
		data, err := os.ReadFile(output.path)
		if !output.enabled {
			if !os.IsNotExist(err) {
				t.Fatalf("disabled path %s: %v", output.path, err)
			}
			continue
		}
		count++
		if err != nil {
			t.Fatal(err)
		}
		path, err := filepath.Rel(g.OutputDir, output.path)
		if err != nil {
			t.Fatal(err)
		}
		if manifest.Files[filepath.ToSlash(path)] != featureDigest(data) {
			t.Fatalf("manifest digest mismatch: %s", path)
		}
	}
	if len(manifest.Files) != count {
		t.Fatalf("manifest tracks unexpected files: %+v", manifest.Files)
	}
	if _, err := os.Stat(filepath.Join(g.OutputDir, featureJournalName)); !os.IsNotExist(err) {
		t.Fatalf("journal not cleaned up: %v", err)
	}
	root := filepath.Join(g.OutputDir, "..", "..")
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(entry.Name(), ".fabrica-tmp") {
			return fmt.Errorf("staging left behind: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestFeatureTransactionEveryFilesystemBoundary(t *testing.T) {
	for _, initial := range []bool{false, true} {
		t.Run(fmt.Sprintf("initial=%t", initial), func(t *testing.T) {
			reference := transactionFixture(t, initial)
			transactionSelection(reference, !initial)
			type operation struct{ kind, path string }
			var operations []operation
			reference.featureTransactionHook = func(kind, path string) error { operations = append(operations, operation{kind, path}); return nil }
			if err := reference.GenerateFeatures(); err != nil {
				t.Fatal(err)
			}
			for index, operation := range operations {
				t.Run(fmt.Sprintf("%03d/%s/%s", index, operation.kind, operation.path), func(t *testing.T) {
					g := transactionFixture(t, initial)
					before := transactionSnapshot(t, g)
					transactionSelection(g, !initial)
					calls := 0
					injected := errors.New("injected filesystem failure")
					g.featureTransactionHook = func(kind, path string) error {
						defer func() { calls++ }()
						if calls == index {
							if kind != operation.kind || path != operation.path {
								t.Fatalf("unstable operation sequence: %s %s, expected %+v", kind, path, operation)
							}
							return injected
						}
						return nil
					}
					if err := g.GenerateFeatures(); !errors.Is(err, injected) {
						t.Fatalf("injection not observed: %v", err)
					}
					info, err := g.InspectFeatureRecovery()
					if err != nil {
						t.Fatalf("unrecoverable failure at %+v: %v", operation, err)
					}
					if info == nil {
						// Before durable intent, outputs are unchanged. After journal cleanup,
						// all outputs are already committed. No intermediate state is valid.
						after := transactionSnapshot(t, g)
						if !reflect.DeepEqual(before, after) {
							assertTransactionComplete(t, g)
						}
					}
					g.featureTransactionHook = nil
					if err := g.GenerateFeatures(); err != nil {
						t.Fatalf("retry: %v", err)
					}
					assertTransactionComplete(t, g)
				})
			}
		})
	}
}

func TestFeatureTransactionShortWritesAndDiskFull(t *testing.T) {
	reference := transactionFixture(t, false)
	transactionSelection(reference, true)
	paths := []string{featureJournalName, featureManifestName}
	for _, output := range reference.featureOutputs() {
		if output.enabled {
			paths = append(paths, filepath.Base(output.path))
		}
	}
	for _, name := range paths {
		for _, failure := range []error{nil, syscall.ENOSPC} {
			t.Run(fmt.Sprintf("%s/%v", name, failure), func(t *testing.T) {
				g := transactionFixture(t, false)
				beforeManifest, err := os.ReadFile(filepath.Join(g.OutputDir, featureManifestName))
				if err != nil {
					t.Fatal(err)
				}
				transactionSelection(g, true)
				var failed bool
				g.featureTransactionWrite = func(f *os.File, data []byte) (int, error) {
					if filepath.Base(f.Name()) == name+".fabrica-tmp" && !failed {
						failed = true
						n, err := f.Write(data[:len(data)/2])
						if err != nil {
							return n, err
						}
						return n, failure
					}
					return f.Write(data)
				}
				err = g.GenerateFeatures()
				want := failure
				if want == nil {
					want = io.ErrShortWrite
				}
				if !failed || !errors.Is(err, want) {
					t.Fatalf("short write: %v", err)
				}
				liveManifest, err := os.ReadFile(filepath.Join(g.OutputDir, featureManifestName))
				if err != nil || !bytes.Equal(beforeManifest, liveManifest) {
					t.Fatalf("live manifest truncated: %v", err)
				}
				g.featureTransactionWrite = nil
				if err := g.GenerateFeatures(); err != nil {
					t.Fatal(err)
				}
				assertTransactionComplete(t, g)
			})
		}
	}
}

func interruptTransaction(t *testing.T, g *Generator) {
	t.Helper()
	g.featureTransactionHook = func(kind, path string) error {
		if kind == "after-rename" && filepath.Base(path) == "features_generated.go" {
			return errors.New("interrupted")
		}
		return nil
	}
	if err := g.GenerateFeatures(); err == nil {
		t.Fatal("expected interruption")
	}
	g.featureTransactionHook = nil
	if info, err := g.InspectFeatureRecovery(); err != nil || info == nil {
		t.Fatalf("missing pending recovery: %+v %v", info, err)
	}
}

func TestFeatureTransactionRejectsUserEditsAndCorruption(t *testing.T) {
	for _, kind := range []string{"output-edit", "output-deleted", "unexpected-creation", "manifest-edit", "path-escape", "duplicate", "missing-output", "bad-digest", "bad-data", "unknown-format", "unknown-field", "missing-before", "missing-absence", "missing-mode", "noncanonical-config", "truncated", "symlink", "parent-symlink"} {
		t.Run(kind, func(t *testing.T) {
			g := transactionFixture(t, false)
			transactionSelection(g, true)
			interruptTransaction(t, g)
			journalPath := filepath.Join(g.OutputDir, featureJournalName)
			encoded, err := os.ReadFile(journalPath)
			if err != nil {
				t.Fatal(err)
			}
			var tx featureTransaction
			if err := json.Unmarshal(encoded, &tx); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "output-edit":
				if err := os.WriteFile(filepath.Join(g.OutputDir, "features_generated.go"), []byte("// user edit"), 0644); err != nil {
					t.Fatal(err)
				}
			case "output-deleted":
				if err := os.Remove(filepath.Join(g.OutputDir, "runtime_helpers_generated.go")); err != nil {
					t.Fatal(err)
				}
			case "unexpected-creation":
				if err := os.WriteFile(filepath.Join(g.OutputDir, "auth_helpers_generated.go"), []byte("// user code"), 0644); err != nil {
					t.Fatal(err)
				}
			case "manifest-edit":
				if err := os.WriteFile(filepath.Join(g.OutputDir, featureManifestName), []byte("{}"), 0644); err != nil {
					t.Fatal(err)
				}
			case "path-escape":
				tx.Files[0].Path = "../../outside.go"
			case "duplicate":
				tx.Files[1] = tx.Files[0]
			case "missing-output":
				tx.Files = tx.Files[:len(tx.Files)-1]
			case "bad-digest":
				tx.Files[0].After.Digest = strings.Repeat("0", 64)
			case "bad-data":
				tx.Files[0].Data = []byte("customization")
			case "unknown-format":
				tx.Format = 99
			case "unknown-field":
				encoded = append(encoded[:len(encoded)-1], []byte(`,"Unexpected":true}`)...)
			case "missing-before", "missing-absence", "missing-mode":
				var raw map[string]interface{}
				if err := json.Unmarshal(encoded, &raw); err != nil {
					t.Fatal(err)
				}
				files := raw["Files"].([]interface{})
				switch kind {
				case "missing-before":
					delete(files[0].(map[string]interface{}), "Before")
				case "missing-absence":
					delete(files[2].(map[string]interface{})["Before"].(map[string]interface{}), "Exists")
				case "missing-mode":
					delete(files[0].(map[string]interface{}), "Mode")
				}
				encoded, err = json.Marshal(raw)
				if err != nil {
					t.Fatal(err)
				}
			case "noncanonical-config":
				tx.Config.ValidationMode = ""
			case "truncated":
				encoded = encoded[:len(encoded)/2]
			case "symlink":
				path := filepath.Join(g.OutputDir, "features_generated.go")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("main.go", path); err != nil {
					t.Fatal(err)
				}
			case "parent-symlink":
				root := filepath.Join(g.OutputDir, "..", "..")
				if err := os.MkdirAll(filepath.Join(root, "application"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("application", filepath.Join(root, "internal")); err != nil {
					t.Fatal(err)
				}
			}
			if kind != "unknown-field" && kind != "truncated" && kind != "missing-before" && kind != "missing-absence" && kind != "missing-mode" {
				encoded, err = json.Marshal(tx)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(journalPath, encoded, 0600); err != nil {
				t.Fatal(err)
			}
			before := snapshotTransactionTree(t, g.OutputDir)
			if err := g.GenerateFeatures(); err == nil {
				t.Fatal("unsafe recovery accepted")
			}
			after := snapshotTransactionTree(t, g.OutputDir)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("rejected recovery mutated files or recovery evidence")
			}
		})
	}
}

func snapshotTransactionTree(t *testing.T, server string) map[string]string {
	t.Helper()
	root := filepath.Join(server, "..", "..")
	result := make(map[string]string)
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			result[path] = "symlink:" + link
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[path] = string(data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestFeatureTransactionChangedSelectionAndReadOnlyPreflight(t *testing.T) {
	g := transactionFixture(t, false)
	transactionSelection(g, true)
	captured := g.Config.Features
	interruptTransaction(t, g)
	snapshot := snapshotTransactionTree(t, g.OutputDir)
	if err := g.CheckFeatureGeneration(true); err == nil || !strings.Contains(err.Error(), "recovery pending") {
		t.Fatalf("preview: %v", err)
	}
	if err := g.PrepareFeatureGeneration(false); err == nil {
		t.Fatal("selective recovered")
	}
	if !reflect.DeepEqual(snapshot, snapshotTransactionTree(t, g.OutputDir)) {
		t.Fatal("read-only preflight changed files")
	}
	// A different requested selection must not leak into the saved intent.
	transactionSelection(g, false)
	writes, _, err := g.FeatureImpact()
	if err == nil || !slices.Contains(writes, "cmd/server/metrics_generated.go") {
		t.Fatalf("pending impact mixed selections: %v %v", writes, err)
	}
	if !reflect.DeepEqual(snapshot, snapshotTransactionTree(t, g.OutputDir)) {
		t.Fatal("pending impact modified files")
	}
	if err := g.PrepareFeatureGeneration(true); err != nil {
		t.Fatal(err)
	}
	var manifest featureManifest
	data, err := os.ReadFile(filepath.Join(g.OutputDir, featureManifestName))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Config != captured {
		t.Fatalf("recovery mixed request configs: %+v", manifest.Config)
	}
	if err := g.GenerateFeatures(); err != nil {
		t.Fatal(err)
	}
	assertTransactionComplete(t, g)
}

func TestFeatureTransactionStableIdentityAndTimestamps(t *testing.T) {
	g := transactionFixture(t, true)
	before := transactionSnapshot(t, g)
	mtimes := map[string]time.Time{}
	for path, data := range before {
		if data != nil {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			mtimes[path] = info.ModTime()
		}
	}
	g.Version = "different"
	g.Commit = "new-commit"
	time.Sleep(5 * time.Millisecond)
	if err := g.GenerateFeatures(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, transactionSnapshot(t, g)) {
		t.Fatal("identity/timestamp-only regeneration changed bytes")
	}
	for path, stamp := range mtimes {
		info, err := os.Stat(path)
		if err != nil || !stamp.Equal(info.ModTime()) {
			t.Fatalf("no-op replaced %s: %v", path, err)
		}
	}
}

// This helper is also invoked by the checkout-built CLI recovery integration.
func TestFeatureTransactionCrashChild(t *testing.T) {
	dir := os.Getenv("FABRICA_TRANSACTION_TEST_SERVER")
	if dir == "" {
		t.Skip("subprocess crash helper")
	}
	g := NewGenerator(dir, "main", os.Getenv("FABRICA_TRANSACTION_TEST_MODULE"))
	if err := g.LoadTemplates(); err != nil {
		t.Fatal(err)
	}
	if config := os.Getenv("FABRICA_TRANSACTION_TEST_CONFIG"); config != "" {
		var f FeatureConfig
		if err := json.Unmarshal([]byte(config), &f); err != nil {
			t.Fatal(err)
		}
		g.SetFeatures(f)
	} else {
		transactionSelection(g, true)
	}
	if os.Getenv("FABRICA_TRANSACTION_TEST_COMPLETE") == "true" {
		fmt.Println("ready")
		if err := g.GenerateFeatures(); err != nil {
			t.Fatal(err)
		}
		return
	}
	point := os.Getenv("FABRICA_TRANSACTION_TEST_CRASH")
	g.featureTransactionHook = func(kind, path string) error {
		if kind+":"+filepath.Base(path) == point {
			os.Exit(91)
		}
		return nil
	}
	if err := g.GenerateFeatures(); err != nil {
		t.Fatal(err)
	}
	t.Fatal("crash hook did not run")
}

func TestFeatureTransactionProcessInterruption(t *testing.T) {
	for _, point := range []string{"before-rename:" + featureJournalName, "before-write:features_generated.go", "after-write:features_generated.go", "after-rename:features_generated.go", "before-remove:" + featureJournalName} {
		t.Run(point, func(t *testing.T) {
			g := transactionFixture(t, false)
			mainPath := filepath.Join(g.OutputDir, "main.go")
			main, err := os.ReadFile(mainPath)
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestFeatureTransactionCrashChild$")
			cmd.Env = append(os.Environ(), "FABRICA_TRANSACTION_TEST_SERVER="+g.OutputDir, "FABRICA_TRANSACTION_TEST_MODULE="+g.ModulePath, "FABRICA_TRANSACTION_TEST_CRASH="+point)
			out, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 91 {
				t.Fatalf("child exit: %v\n%s", err, out)
			}
			transactionSelection(g, true)
			if err := g.GenerateFeatures(); err != nil {
				t.Fatal(err)
			}
			assertTransactionComplete(t, g)
			if err := g.GenerateFeatures(); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(mainPath)
			if err != nil || !bytes.Equal(main, got) {
				t.Fatalf("recovery changed user main: %v", err)
			}
		})
	}
}

func TestFeatureTransactionConcurrentGenerators(t *testing.T) {
	g := transactionFixture(t, false)
	transactionSelection(g, true)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	g.featureTransactionHook = func(kind, path string) error {
		if kind == "before-rename" && filepath.Base(path) == featureJournalName {
			once.Do(func() { close(entered); <-release })
		}
		return nil
	}
	first := make(chan error, 1)
	go func() { first <- g.GenerateFeatures() }()
	<-entered
	other := NewGenerator(g.OutputDir, g.PackageName, g.ModulePath)
	if err := other.LoadTemplates(); err != nil {
		t.Fatal(err)
	}
	transactionSelection(other, true)
	second := make(chan error, 1)
	go func() { second <- other.GenerateFeatures() }()
	select {
	case err := <-second:
		close(release)
		t.Fatalf("concurrent generator bypassed lock: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	assertTransactionComplete(t, other)
}

func TestGenerateAllRecoversFeaturesBeforeServerWrites(t *testing.T) {
	g := transactionFixture(t, false)
	t.Chdir(filepath.Clean(filepath.Join(g.OutputDir, "..", "..")))
	f := g.Config.Features
	f.MetricsEnabled = true
	g.SetFeatures(f)
	interruptTransaction(t, g)
	if err := g.GenerateAll(); err != nil {
		t.Fatal(err)
	}
	assertTransactionComplete(t, g)
	if err := g.CheckFeatureGeneration(false); err != nil {
		t.Fatal(err)
	}
}

func TestFeatureTransactionLockSpansProcesses(t *testing.T) {
	g := transactionFixture(t, false)
	var child *exec.Cmd
	done := make(chan error, 1)
	if err := g.withFeatureLock(func(*os.Root) error {
		child = exec.Command(os.Args[0], "-test.run=^TestFeatureTransactionCrashChild$")
		child.Env = append(os.Environ(), "FABRICA_TRANSACTION_TEST_SERVER="+g.OutputDir, "FABRICA_TRANSACTION_TEST_MODULE="+g.ModulePath, "FABRICA_TRANSACTION_TEST_COMPLETE=true")
		stdout, err := child.StdoutPipe()
		if err != nil {
			return err
		}
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			return err
		}
		go func() { done <- child.Wait() }()
		ready := make(chan error, 1)
		go func() {
			scanner := bufio.NewScanner(stdout)
			if !scanner.Scan() {
				ready <- fmt.Errorf("child did not start: %v", scanner.Err())
				return
			}
			if scanner.Text() != "ready" {
				ready <- fmt.Errorf("unexpected child message %q", scanner.Text())
				return
			}
			ready <- nil
		}()
		select {
		case err := <-ready:
			if err != nil {
				return err
			}
		case <-time.After(10 * time.Second):
			return fmt.Errorf("child startup timed out")
		}
		select {
		case err := <-done:
			return fmt.Errorf("child bypassed lock: %v", err)
		case <-time.After(40 * time.Millisecond):
			return nil
		}
	}); err != nil {
		if child != nil && child.Process != nil {
			_ = child.Process.Kill()
		}
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		_ = child.Process.Kill()
		t.Fatal("child did not acquire released lock")
	}
	transactionSelection(g, true)
	assertTransactionComplete(t, g)
}

func TestFeatureRecoverySyncsObservedRenamesBeforeFurtherMutation(t *testing.T) {
	g := transactionFixture(t, false)
	transactionSelection(g, true)
	interruptTransaction(t, g)
	manifestPath := filepath.Join(g.OutputDir, featureManifestName)
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	helperPath := filepath.Join(g.OutputDir, "runtime_helpers_generated.go")
	helper, err := os.ReadFile(helperPath)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	injected := errors.New("directory sync unavailable")
	g.featureTransactionHook = func(kind, path string) error {
		if kind == "before-directory-sync" && path == "cmd/server" {
			count++
			if count == 2 {
				return injected
			}
		}
		return nil
	}
	if err := g.PrepareFeatureGeneration(true); !errors.Is(err, injected) {
		t.Fatalf("recovered rename was not synced: %v", err)
	}
	for path, want := range map[string][]byte{manifestPath: manifest, helperPath: helper} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("recovery mutated %s before confirming durability: %v", path, err)
		}
	}
	g.featureTransactionHook = nil
	if err := g.GenerateFeatures(); err != nil {
		t.Fatal(err)
	}
	assertTransactionComplete(t, g)
}

func TestFeatureRecoverySyncsObservedRemoval(t *testing.T) {
	for _, removeParent := range []bool{false, true} {
		t.Run(fmt.Sprintf("missingParent=%t", removeParent), func(t *testing.T) {
			g := transactionFixture(t, true)
			transactionSelection(g, false)
			g.featureTransactionHook = func(kind, path string) error {
				if kind == "after-remove" && filepath.Base(path) == "validation_middleware_generated.go" {
					return errors.New("interrupted removal")
				}
				return nil
			}
			if err := g.GenerateFeatures(); err == nil {
				t.Fatal("expected interruption")
			}
			g.featureTransactionHook = nil
			middleware := filepath.Join(g.OutputDir, "..", "..", "internal", "middleware")
			if removeParent {
				if err := os.RemoveAll(middleware); err != nil {
					t.Fatal(err)
				}
			} else {
				injected := errors.New("directory sync unavailable")
				g.featureTransactionHook = func(kind, path string) error {
					if kind == "before-directory-sync" && path == "internal/middleware" {
						return injected
					}
					return nil
				}
				if err := g.PrepareFeatureGeneration(true); !errors.Is(err, injected) {
					t.Fatalf("observed removal was not synced: %v", err)
				}
				if _, err := os.Stat(filepath.Join(middleware, "conditional_middleware_generated.go")); err != nil {
					t.Fatalf("subsequent deletion happened before sync: %v", err)
				}
			}
			g.featureTransactionHook = nil
			if err := g.GenerateFeatures(); err != nil {
				t.Fatal(err)
			}
			assertTransactionComplete(t, g)
		})
	}
}

func TestFeaturePreparationInterruptionThenNoopSelection(t *testing.T) {
	g := transactionFixture(t, false)
	before := transactionSnapshot(t, g)
	cmd := exec.Command(os.Args[0], "-test.run=^TestFeatureTransactionCrashChild$")
	cmd.Env = append(os.Environ(), "FABRICA_TRANSACTION_TEST_SERVER="+g.OutputDir, "FABRICA_TRANSACTION_TEST_MODULE="+g.ModulePath, "FABRICA_TRANSACTION_TEST_CRASH=before-rename:"+featureJournalName)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 91 {
		t.Fatalf("child exit: %v\n%s", err, out)
	}
	temporary := filepath.Join(g.OutputDir, featureTemporaryPath(featureJournalName))
	if _, err := os.Stat(temporary); err != nil {
		t.Fatal(err)
	}
	injected := errors.New("cleanup interrupted")
	g.featureTransactionHook = func(kind, path string) error {
		if kind == "before-remove" && filepath.Base(path) == featureTemporaryPath(featureJournalName) {
			return injected
		}
		return nil
	}
	if err := g.GenerateFeatures(); !errors.Is(err, injected) {
		t.Fatalf("unpublished staging cleanup was skipped: %v", err)
	}
	if !reflect.DeepEqual(before, transactionSnapshot(t, g)) {
		t.Fatal("unpublished intent changed live outputs")
	}
	g.featureTransactionHook = nil
	if err := g.GenerateFeatures(); err != nil {
		t.Fatal(err)
	}
	assertTransactionComplete(t, g)
	if !reflect.DeepEqual(before, transactionSnapshot(t, g)) {
		t.Fatal("discarding unpublished intent changed the no-op selection")
	}
}
