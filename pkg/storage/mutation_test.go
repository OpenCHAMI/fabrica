// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFileMutationSerializesSeparateBackends(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	first, err := NewFileBackend(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewFileBackend(filepath.Join(root, "."))
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Save(ctx, "Node", "node-1", json.RawMessage(`{"value":0}`)); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	failures := make(chan error, 2)
	for _, backend := range []*FileBackend{first, second} {
		wg.Go(func() {
			<-start
			for i := 0; i < 30; i++ {
				_, err := backend.Mutate(ctx, "Node", "node-1", func(raw json.RawMessage) (json.RawMessage, bool, error) {
					var counter struct {
						Value int `json:"value"`
					}
					if err := json.Unmarshal(raw, &counter); err != nil {
						return nil, false, err
					}
					counter.Value++
					next, err := json.Marshal(counter)
					return next, false, err
				})
				if err != nil {
					failures <- err
					return
				}
			}
		})
	}
	close(start)
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	raw, err := first.Load(ctx, "Node", "node-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"value":60}` {
		t.Fatalf("lost update: %s", raw)
	}
}

func TestFileMutationRollbackAndDelete(t *testing.T) {
	ctx := context.Background()
	backend, err := NewFileBackend(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	initial := json.RawMessage(`{"value":"initial"}`)
	if err := backend.Save(ctx, "Node", "node-1", initial); err != nil {
		t.Fatal(err)
	}
	rejected := errors.New("precondition failed")
	for _, fn := range []Mutation{
		func(json.RawMessage) (json.RawMessage, bool, error) { return nil, true, rejected },
		func(json.RawMessage) (json.RawMessage, bool, error) { return json.RawMessage(`invalid`), false, nil },
	} {
		if _, err := backend.Mutate(ctx, "Node", "node-1", fn); err == nil {
			t.Fatal("invalid mutation succeeded")
		}
		raw, err := backend.Load(ctx, "Node", "node-1")
		if err != nil || string(raw) != string(initial) {
			t.Fatalf("changed after failure: %s %v", raw, err)
		}
	}
	deleted, err := backend.Mutate(ctx, "Node", "node-1", func(json.RawMessage) (json.RawMessage, bool, error) { return nil, true, nil })
	if err != nil || string(deleted) != string(initial) {
		t.Fatalf("delete result %s %v", deleted, err)
	}
	if _, err := backend.Load(ctx, "Node", "node-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete not committed: %v", err)
	}
}

type mutationVersionRegistry struct{}

func (mutationVersionRegistry) GetDefaultVersion(string) string               { return "v1" }
func (mutationVersionRegistry) GetVersion(string, string) (VersionInfo, bool) { return nil, false }

func TestFileVersionMethodsDoNotRelock(t *testing.T) {
	backend, err := NewFileBackend(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	backend.SetVersionRegistry(mutationVersionRegistry{})
	done := make(chan error, 1)
	go func() {
		ctx := context.Background()
		if err := backend.SaveWithVersion(ctx, "Node", "node-1", json.RawMessage(`{"value":"initial"}`), "v1"); err != nil {
			done <- err
			return
		}
		if _, _, err := backend.LoadWithVersion(ctx, "Node", "node-1", "v1"); err != nil {
			done <- err
			return
		}
		_, err := backend.LoadAllWithVersion(ctx, "Node", "v1")
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("versioned method deadlocked by reacquiring its mutex")
	}
}

func TestFileMutationSerializesOrdinaryWriters(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(fmt.Sprintf("delete=%t", remove), func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			first, err := NewFileBackend(root)
			if err != nil {
				t.Fatal(err)
			}
			alias := filepath.Join(t.TempDir(), "alias")
			if err := os.Symlink(root, alias); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			second, err := NewFileBackend(alias)
			if err != nil {
				t.Fatal(err)
			}
			if err := first.Save(ctx, "Node", "node-1", json.RawMessage(`{"value":"initial"}`)); err != nil {
				t.Fatal(err)
			}
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			defer unblock()
			mutated, written := make(chan error, 1), make(chan error, 1)
			go func() {
				_, err := first.Mutate(ctx, "Node", "node-1", func(json.RawMessage) (json.RawMessage, bool, error) {
					close(entered)
					<-release
					return json.RawMessage(`{"value":"mutation"}`), false, nil
				})
				mutated <- err
			}()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("mutation never entered callback")
			}
			go func() {
				if remove {
					written <- second.Delete(ctx, "Node", "node-1")
				} else {
					written <- second.Save(ctx, "Node", "node-1", json.RawMessage(`{"value":"ordinary writer"}`))
				}
			}()
			select {
			case err := <-written:
				t.Fatalf("ordinary writer bypassed mutation lock: %v", err)
			case <-time.After(20 * time.Millisecond):
			}
			unblock()
			if err := <-mutated; err != nil {
				t.Fatal(err)
			}
			if err := <-written; err != nil {
				t.Fatal(err)
			}
			raw, err := first.Load(ctx, "Node", "node-1")
			if remove {
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("deleted resource returned: %s %v", raw, err)
				}
			} else if err != nil || string(raw) != `{"value":"ordinary writer"}` {
				t.Fatalf("ordinary writer lost: %s %v", raw, err)
			}
		})
	}
}

func TestFileMutationCancellationDoesNotWrite(t *testing.T) {
	backend, err := NewFileBackend(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	initial := json.RawMessage(`{"value":"initial"}`)
	if err := backend.Save(context.Background(), "Node", "node-1", initial); err != nil {
		t.Fatal(err)
	}
	for _, cancelBefore := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		if cancelBefore {
			cancel()
		}
		called := false
		_, err := backend.Mutate(ctx, "Node", "node-1", func(json.RawMessage) (json.RawMessage, bool, error) {
			called = true
			cancel()
			return json.RawMessage(`{"value":"canceled"}`), false, nil
		})
		cancel()
		if !errors.Is(err, context.Canceled) || (cancelBefore && called) {
			t.Fatalf("canceled mutation: called=%t error=%v", called, err)
		}
		raw, err := backend.Load(context.Background(), "Node", "node-1")
		if err != nil || string(raw) != string(initial) {
			t.Fatalf("canceled mutation changed data: %s %v", raw, err)
		}
	}
}
