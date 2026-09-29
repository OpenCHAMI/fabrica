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
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	series, err := s.readSeries(name)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(status)
	if err != nil {
		return "", fmt.Errorf("revision: encode series status: %w", err)
	}
	series.ResourceStatus = encoded
	series.Status.UpdatedAt = s.now()
	if err := s.writeSeries(name, series); err != nil {
		return "", err
	}
	return seriesETag(series), nil
}
