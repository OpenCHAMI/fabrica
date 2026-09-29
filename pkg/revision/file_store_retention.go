// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import "context"

// RetireRevision tombstones an unreferenced revision without releasing its name, number, or UID.
func (s *FileStore[T]) RetireRevision(ctx context.Context, name, revisionUID string) (Record[T], error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Record[T]{}, err
	}
	series, err := s.readSeries(name)
	if err != nil {
		return Record[T]{}, err
	}
	if series.Status.DefaultRevisionUID == revisionUID || series.Status.LatestRevisionUID == revisionUID {
		return Record[T]{}, ErrRevisionInUse
	}
	record, err := s.readCommittedRecord(series, revisionUID)
	if err != nil {
		return Record[T]{}, err
	}
	if record.Metadata.RetiredAt != nil {
		return Record[T]{}, ErrRevisionRetired
	}
	retiredAt := s.now()
	record.Metadata.RetiredAt = &retiredAt
	series.Status.UpdatedAt = retiredAt
	if err := s.writeRecord(record); err != nil {
		return Record[T]{}, err
	}
	if err := s.writeSeries(name, series); err != nil {
		return Record[T]{}, err
	}
	return record, nil
}
