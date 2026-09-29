// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	// ErrSeriesExists reports that a revision series already exists.
	ErrSeriesExists = errors.New("revision: series already exists")
	// ErrSeriesNotFound reports that a revision series could not be found.
	ErrSeriesNotFound = errors.New("revision: series not found")
	// ErrRevisionNotFound reports that a revision could not be found.
	ErrRevisionNotFound = errors.New("revision: revision not found")
	// ErrRevisionNameConflict reports that a revision name is bound to different content.
	ErrRevisionNameConflict = errors.New("revision: revision name conflict")
	// ErrPreconditionFailed reports that a compare-and-swap precondition failed.
	ErrPreconditionFailed = errors.New("revision: precondition failed")
	// ErrRevisionRetired reports that a revision has been retired.
	ErrRevisionRetired = errors.New("revision: revision retired")
	// ErrRevisionInUse reports that a revision is still referenced by an alias.
	ErrRevisionInUse = errors.New("revision: revision is referenced by an alias")
)

// ConflictError reports an immutable revision-name binding conflict.
type ConflictError struct {
	ExistingRevisionUID string
	ExistingDigest      string
	RequestedDigest     string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%v: existing revision %s has digest %s, requested %s", ErrRevisionNameConflict, e.ExistingRevisionUID, e.ExistingDigest, e.RequestedDigest)
}

func (e *ConflictError) Unwrap() error { return ErrRevisionNameConflict }

// PreconditionError reports the current series ETag after a failed comparison.
type PreconditionError struct {
	CurrentETag string
}

func (e *PreconditionError) Error() string {
	return fmt.Sprintf("%v: current ETag is %s", ErrPreconditionFailed, e.CurrentETag)
}

func (e *PreconditionError) Unwrap() error { return ErrPreconditionFailed }

// Record combines immutable revision metadata with the stored resource envelope.
type Record[T any] struct {
	Metadata Metadata `json:"metadata"`
	Resource T        `json:"resource"`
}

type fileSeries struct {
	Status         SeriesStatus      `json:"status"`
	RevisionNames  map[string]string `json:"revisionNames,omitempty"`
	RevisionUIDs   []string          `json:"revisionUids"`
	ResourceStatus json.RawMessage   `json:"resourceStatus,omitempty"`
}

var fileStoreLocks sync.Map

// FileStore persists one resource kind's immutable revision series.
type FileStore[T any] struct {
	root string
	mu   *sync.Mutex
	now  func() time.Time
}

// NewFileStore creates a file-backed immutable revision store for one resource kind.
func NewFileStore[T any](baseDir, resourceKind string) (*FileStore[T], error) {
	if resourceKind == "" || strings.ContainsAny(resourceKind, `/\\.`) {
		return nil, fmt.Errorf("revision: invalid resource kind %q", resourceKind)
	}
	root := filepath.Join(baseDir, "revisions", strings.ToLower(resourceKind))
	for _, dir := range []string{filepath.Join(root, "series"), filepath.Join(root, "records")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("revision: create store directory: %w", err)
		}
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("revision: resolve store directory: %w", err)
	}
	lock, _ := fileStoreLocks.LoadOrStore(canonicalRoot, &sync.Mutex{})
	return &FileStore[T]{root: canonicalRoot, mu: lock.(*sync.Mutex), now: func() time.Time { return time.Now().UTC() }}, nil
}

// CreateSeries creates a named series and its first immutable revision.
func (s *FileStore[T]) CreateSeries(ctx context.Context, name, revisionName, digest string, resource T) (Record[T], string, error) {
	return s.createSeries(ctx, name, revisionName, digest, resource, nil)
}

// CreateSeriesWithStatus creates a series while initializing its series-level status.
func (s *FileStore[T]) CreateSeriesWithStatus(ctx context.Context, name, revisionName, digest string, resource T, status any) (Record[T], string, error) {
	encodedStatus, err := json.Marshal(status)
	if err != nil {
		return Record[T]{}, "", fmt.Errorf("revision: encode series status: %w", err)
	}
	return s.createSeries(ctx, name, revisionName, digest, resource, encodedStatus)
}

func (s *FileStore[T]) createSeries(ctx context.Context, name, revisionName, digest string, resource T, status json.RawMessage) (Record[T], string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Record[T]{}, "", err
	}
	if _, err := os.Stat(s.seriesPath(name)); err == nil {
		return Record[T]{}, "", ErrSeriesExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return Record[T]{}, "", fmt.Errorf("revision: inspect series: %w", err)
	}

	record, err := s.newRecord(name, revisionName, digest, 1, resource)
	if err != nil {
		return Record[T]{}, "", err
	}
	series := fileSeries{
		Status:         SeriesStatus{Name: name, DefaultRevisionUID: record.Metadata.UID, LatestRevisionUID: record.Metadata.UID, RevisionCount: 1, UpdatedAt: record.Metadata.CreatedAt},
		RevisionNames:  map[string]string{},
		RevisionUIDs:   []string{record.Metadata.UID},
		ResourceStatus: status,
	}
	if revisionName != "" {
		series.RevisionNames[revisionName] = record.Metadata.UID
	}
	if err := s.writeRecord(record); err != nil {
		return Record[T]{}, "", err
	}
	if err := s.writeSeries(name, series); err != nil {
		return Record[T]{}, "", err
	}
	return record, seriesETag(series), nil
}

// GetSeries returns series state and its comparison ETag.
func (s *FileStore[T]) GetSeries(ctx context.Context, name string) (SeriesStatus, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return SeriesStatus{}, "", err
	}
	series, err := s.readSeries(name)
	if err != nil {
		return SeriesStatus{}, "", err
	}
	return series.Status, seriesETag(series), nil
}

// GetRevision returns one immutable revision and verifies its series linkage.
func (s *FileStore[T]) GetRevision(ctx context.Context, name, uid string) (Record[T], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Record[T]{}, err
	}
	series, err := s.readSeries(name)
	if err != nil {
		return Record[T]{}, err
	}
	return s.readAuthoritativeRecord(series, uid)
}

// ListRevisions returns a series' revisions in ascending series-local order.
func (s *FileStore[T]) ListRevisions(ctx context.Context, name string) ([]Record[T], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	series, err := s.readSeries(name)
	if err != nil {
		return nil, err
	}
	records := make([]Record[T], 0, len(series.RevisionUIDs))
	for _, uid := range series.RevisionUIDs {
		record, err := s.readCommittedRecord(series, uid)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func seriesETag(series fileSeries) string {
	keys := make([]string, 0, len(series.RevisionNames))
	for key := range series.RevisionNames {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var canonical strings.Builder
	fmt.Fprintf(&canonical, "%s\x00%s\x00%s\x00%d\x00%s", series.Status.Name, series.Status.DefaultRevisionUID, series.Status.LatestRevisionUID, series.Status.RevisionCount, series.Status.UpdatedAt.Format(time.RFC3339Nano))
	canonical.Write(series.ResourceStatus)
	for _, uid := range series.RevisionUIDs {
		fmt.Fprintf(&canonical, "\x00%s", uid)
	}
	for _, key := range keys {
		fmt.Fprintf(&canonical, "\x00%s\x00%s", key, series.RevisionNames[key])
	}
	sum := sha256.Sum256([]byte(canonical.String()))
	return `"sha256:` + hex.EncodeToString(sum[:]) + `"`
}
