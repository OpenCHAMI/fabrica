// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

const genericAnnotationSource = `package test

// +fabrica:resource
type Widget struct{}
`

func TestParseFileAnnotations_cache_concurrent_source_replacements_are_race_safe(t *testing.T) {
	// Given
	resetGlobalCache(t)
	directory := t.TempDir()
	filename := filepath.Join(directory, "types.go")
	if err := os.WriteFile(filename, []byte(completeAnnotationSource), 0o644); err != nil {
		t.Fatal(err)
	}
	const readerCount = 8
	const iterations = 20
	errs := make(chan error, readerCount*iterations+iterations)
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(readerCount + 1)

	// When
	for range readerCount {
		go func() {
			defer workers.Done()
			<-start
			for range iterations {
				got, err := ParseFileAnnotations(filename)
				if err != nil {
					errs <- err
					continue
				}
				widget := got["Widget"]
				if widget == nil || !widget.IsResource {
					errs <- fmt.Errorf("incomplete Widget result: %#v", widget)
				}
			}
		}()
	}
	go func() {
		defer workers.Done()
		<-start
		for iteration := range iterations {
			source := completeAnnotationSource
			if iteration%2 == 0 {
				source = genericAnnotationSource
			}
			temporary, err := os.CreateTemp(directory, "types-*.go")
			if err != nil {
				errs <- err
				return
			}
			temporaryName := temporary.Name()
			if _, err := temporary.WriteString(source); err != nil {
				closeErr := temporary.Close()
				removeErr := os.Remove(temporaryName)
				errs <- errors.Join(err, closeErr, removeErr)
				return
			}
			if err := temporary.Close(); err != nil {
				removeErr := os.Remove(temporaryName)
				errs <- errors.Join(err, removeErr)
				return
			}
			if err := os.Rename(temporaryName, filename); err != nil {
				removeErr := os.Remove(temporaryName)
				errs <- errors.Join(err, removeErr)
				return
			}
		}
	}()
	close(start)
	workers.Wait()
	close(errs)

	// Then
	for err := range errs {
		t.Errorf("concurrent source parse: %v", err)
	}
}
