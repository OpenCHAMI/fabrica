// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestFileStoreConcurrentStatusPreconditions(t *testing.T) {
	t.Parallel()
	store, err := NewFileStore[string](t.TempDir(), "BootConfig")
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	_, etag, err := store.CreateSeries(ctx, "compute", "initial", "digest", "content")
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	results := make(chan error, 2)
	for _, value := range []string{"Ready", "Pending"} {
		go func(value string) {
			_, err := store.MutateStatus(ctx, "compute", func(_ json.RawMessage, currentETag string) (json.RawMessage, error) {
				if currentETag != etag {
					return nil, &PreconditionError{CurrentETag: currentETag}
				}
				// A reentrant read must not deadlock during validation.
				var status any
				if err := store.GetStatus(ctx, "compute", &status); err != nil {
					return nil, err
				}
				ready <- struct{}{}
				<-release
				return json.Marshal(value)
			})
			results <- err
		}(value)
	}
	for range 2 {
		<-ready
	}
	close(release)
	succeeded, rejected := 0, 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrPreconditionFailed):
			rejected++
		default:
			t.Fatalf("status mutation: %v", err)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("successes=%d precondition failures=%d", succeeded, rejected)
	}
	records, err := store.ListRevisions(ctx, "compute")
	if err != nil || len(records) != 1 {
		t.Fatalf("status mutation changed revision history: %v, %v", records, err)
	}
}

func TestFileStoreStatusMutationRevalidatesFreshState(t *testing.T) {
	t.Parallel()
	store, err := NewFileStore[string](t.TempDir(), "BootConfig")
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	if _, _, err := store.CreateSeriesWithStatus(ctx, "compute", "initial", "digest", "content", map[string]string{"base": "initial"}); err != nil {
		t.Fatal(err)
	}
	attempts := 0
	_, err = store.MutateStatus(ctx, "compute", func(raw json.RawMessage, _ string) (json.RawMessage, error) {
		attempts++
		var status map[string]string
		if err := json.Unmarshal(raw, &status); err != nil {
			return nil, err
		}
		if attempts == 1 {
			// Simulate a competing write between validation and commit.
			if _, err := store.UpdateStatus(ctx, "compute", map[string]string{"base": "concurrent"}); err != nil {
				return nil, err
			}
		}
		status["phase"] = "Ready"
		return json.Marshal(status)
	})
	if err != nil {
		t.Fatal(err)
	}
	var status map[string]string
	if err := store.GetStatus(ctx, "compute", &status); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || status["base"] != "concurrent" || status["phase"] != "Ready" {
		t.Fatalf("attempts=%d status=%v", attempts, status)
	}
}
