// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// EnsureRevision idempotently binds a revision name to immutable content.
func (s *FileStore[T]) EnsureRevision(ctx context.Context, name, revisionName, digest string, resource T) (Record[T], bool, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Record[T]{}, false, "", err
	}
	series, err := s.readSeries(name)
	if err != nil {
		return Record[T]{}, false, "", err
	}
	if revisionName != "" {
		if uid := series.RevisionNames[revisionName]; uid != "" {
			record, err := s.readCommittedRecord(series, uid)
			if err != nil {
				return Record[T]{}, false, "", err
			}
			if record.Metadata.Digest != digest {
				return Record[T]{}, false, "", &ConflictError{ExistingRevisionUID: uid, ExistingDigest: record.Metadata.Digest, RequestedDigest: digest}
			}
			if record.Metadata.RetiredAt != nil {
				return Record[T]{}, false, "", ErrRevisionRetired
			}
			return record, false, seriesETag(series), nil
		}
	}

	number := series.Status.RevisionCount + 1
	record, err := s.newRecord(name, revisionName, digest, number, resource)
	if err != nil {
		return Record[T]{}, false, "", err
	}
	series.Status.LatestRevisionUID = record.Metadata.UID
	series.Status.RevisionCount = number
	series.Status.UpdatedAt = record.Metadata.CreatedAt
	series.RevisionUIDs = append(series.RevisionUIDs, record.Metadata.UID)
	if revisionName != "" {
		series.RevisionNames[revisionName] = record.Metadata.UID
	}
	if err := s.writeRecord(record); err != nil {
		return Record[T]{}, false, "", err
	}
	if err := s.writeSeries(name, series); err != nil {
		return Record[T]{}, false, "", err
	}
	return record, true, seriesETag(series), nil
}

// Resolve resolves a UID, revision name, default/latest selector, or bare series name.
func (s *FileStore[T]) Resolve(ctx context.Context, reference Reference) (Record[T], error) {
	if err := reference.Validate(); err != nil {
		return Record[T]{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Record[T]{}, err
	}
	series, err := s.readSeries(reference.Name)
	if err != nil {
		return Record[T]{}, err
	}
	uid := reference.UID
	if reference.RevisionName != "" {
		uid = series.RevisionNames[reference.RevisionName]
	}
	if uid == "" {
		switch reference.SelectorOrDefault() {
		case SelectorDefault:
			uid = series.Status.DefaultRevisionUID
		case SelectorLatest:
			uid = series.Status.LatestRevisionUID
		}
	}
	if uid == "" {
		return Record[T]{}, ErrRevisionNotFound
	}
	return s.readAuthoritativeRecord(series, uid)
}

// PromoteDefault atomically moves the default alias when the supplied ETag matches.
func (s *FileStore[T]) PromoteDefault(ctx context.Context, name, revisionUID, ifMatch string) (SeriesStatus, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return SeriesStatus{}, "", err
	}
	series, err := s.readSeries(name)
	if err != nil {
		return SeriesStatus{}, "", err
	}
	currentETag := seriesETag(series)
	if ifMatch == "" || ifMatch != currentETag {
		return SeriesStatus{}, currentETag, &PreconditionError{CurrentETag: currentETag}
	}
	if _, err := s.readAuthoritativeRecord(series, revisionUID); err != nil {
		return SeriesStatus{}, currentETag, err
	}
	series.Status.DefaultRevisionUID = revisionUID
	series.Status.UpdatedAt = s.now()
	if err := s.writeSeries(name, series); err != nil {
		return SeriesStatus{}, currentETag, err
	}
	return series.Status, seriesETag(series), nil
}

func (s *FileStore[T]) newRecord(seriesName, revisionName, digest string, number uint64, resource T) (Record[T], error) {
	uid, err := NewUID()
	if err != nil {
		return Record[T]{}, err
	}
	return Record[T]{Metadata: Metadata{UID: uid, SeriesName: seriesName, RevisionName: revisionName, Number: number, Digest: digest, CreatedAt: s.now()}, Resource: resource}, nil
}

func (s *FileStore[T]) seriesPath(name string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(name))
	return filepath.Join(s.root, "series", encoded+".json")
}

func (s *FileStore[T]) recordPath(uid string) string {
	return filepath.Join(s.root, "records", uid+".json")
}

func (s *FileStore[T]) readSeries(name string) (fileSeries, error) {
	var series fileSeries
	if err := readJSONFile(s.seriesPath(name), &series); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fileSeries{}, ErrSeriesNotFound
		}
		return fileSeries{}, err
	}
	if series.RevisionNames == nil {
		series.RevisionNames = map[string]string{}
	}
	if series.RevisionUIDs == nil {
		series.RevisionUIDs = authoritativeUIDsFromLegacySeries(series)
	}
	return series, nil
}

func (s *FileStore[T]) readRecord(uid string) (Record[T], error) {
	var record Record[T]
	if err := readJSONFile(s.recordPath(uid), &record); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Record[T]{}, ErrRevisionNotFound
		}
		return Record[T]{}, err
	}
	return record, nil
}

func (s *FileStore[T]) readLinkedRecord(name, uid string) (Record[T], error) {
	record, err := s.readRecord(uid)
	if err != nil {
		return Record[T]{}, err
	}
	if record.Metadata.SeriesName != name {
		return Record[T]{}, ErrRevisionNotFound
	}
	return record, nil
}

func (s *FileStore[T]) readAuthoritativeRecord(series fileSeries, uid string) (Record[T], error) {
	record, err := s.readCommittedRecord(series, uid)
	if err != nil {
		return Record[T]{}, err
	}
	if record.Metadata.RetiredAt != nil {
		return Record[T]{}, ErrRevisionRetired
	}
	return record, nil
}

func (s *FileStore[T]) readCommittedRecord(series fileSeries, uid string) (Record[T], error) {
	committed := false
	for _, candidate := range series.RevisionUIDs {
		if candidate == uid {
			committed = true
			break
		}
	}
	if !committed {
		return Record[T]{}, ErrRevisionNotFound
	}
	return s.readLinkedRecord(series.Status.Name, uid)
}

func authoritativeUIDsFromLegacySeries(series fileSeries) []string {
	seen := map[string]bool{}
	uids := make([]string, 0, len(series.RevisionNames)+2)
	for _, uid := range []string{series.Status.DefaultRevisionUID, series.Status.LatestRevisionUID} {
		if uid != "" && !seen[uid] {
			seen[uid] = true
			uids = append(uids, uid)
		}
	}
	for _, uid := range series.RevisionNames {
		if uid != "" && !seen[uid] {
			seen[uid] = true
			uids = append(uids, uid)
		}
	}
	return uids
}

func (s *FileStore[T]) writeSeries(name string, series fileSeries) error {
	return writeJSONFile(s.seriesPath(name), series)
}

func (s *FileStore[T]) writeRecord(record Record[T]) error {
	return writeJSONFile(s.recordPath(record.Metadata.UID), record)
}

func readJSONFile(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("revision: decode %s: %w", path, err)
	}
	return nil
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("revision: encode %s: %w", path, err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".revision-*.tmp")
	if err != nil {
		return fmt.Errorf("revision: create temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("revision: write temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("revision: close temporary file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("revision: replace %s: %w", path, err)
	}
	return nil
}
