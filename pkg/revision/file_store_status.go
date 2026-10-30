// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetStatus decodes the mutable series-level status into destination.
func (s *FileStore[T]) GetStatus(ctx context.Context, name string, destination any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	series, err := s.readSeries(name)
	if err != nil {
		return err
	}
	if len(series.ResourceStatus) == 0 {
		return nil
	}
	if err := json.Unmarshal(series.ResourceStatus, destination); err != nil {
		return fmt.Errorf("revision: decode series status: %w", err)
	}
	return nil
}

// UpdateStatus replaces mutable series-level status without changing a revision.
func (s *FileStore[T]) UpdateStatus(ctx context.Context, name string, status any) (string, error) {
	encoded, err := json.Marshal(status)
	if err != nil {
		return "", fmt.Errorf("revision: encode series status: %w", err)
	}
	return s.MutateStatus(ctx, name, func(json.RawMessage, string) (json.RawMessage, error) {
		return encoded, nil
	})
}

// MutateStatus validates a candidate outside the store lock and commits only
// while the complete series ETag still matches the observed state. The callback
// may run again after contention and must not publish events or mutate the store.
func (s *FileStore[T]) MutateStatus(ctx context.Context, name string, mutate func(json.RawMessage, string) (json.RawMessage, error)) (string, error) {
	for range 8 {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		s.mu.Lock()
		series, err := s.readSeries(name)
		s.mu.Unlock()
		if err != nil {
			return "", err
		}
		observed := seriesETag(series)
		encoded, err := mutate(series.ResourceStatus, observed)
		if err != nil {
			return "", err
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}

		s.mu.Lock()
		current, err := s.readSeries(name)
		if err != nil {
			s.mu.Unlock()
			return "", err
		}
		if seriesETag(current) != observed {
			s.mu.Unlock()
			continue
		}
		current.ResourceStatus = encoded
		current.Status.UpdatedAt = s.now()
		err = s.writeSeries(name, current)
		s.mu.Unlock()
		if err != nil {
			return "", err
		}
		return seriesETag(current), nil
	}
	return "", fmt.Errorf("revision: series kept changing during status validation: %w", ErrPreconditionFailed)
}
