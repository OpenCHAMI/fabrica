// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func (s *FileStore[T]) readCanonicalSeries(name string) (series fileSeries, present bool, resultErr error) {
	root, err := os.OpenRoot(filepath.Join(s.root, "series"))
	if err != nil {
		return fileSeries{}, false, fmt.Errorf("revision: open series directory: %w", err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("revision: close series directory: %w", err))
		}
	}()
	filename := filepath.Base(s.seriesPath(name))
	entry, err := root.Lstat(filename)
	if errors.Is(err, os.ErrNotExist) {
		return fileSeries{}, false, nil
	}
	if err != nil {
		return fileSeries{}, false, fmt.Errorf("revision: inspect canonical series: %w", err)
	}
	if !entry.Mode().IsRegular() {
		return fileSeries{}, true, fmt.Errorf("revision: canonical series %q is not a regular file", name)
	}
	file, err := root.Open(filename)
	if err != nil {
		return fileSeries{}, true, fmt.Errorf("revision: open canonical series: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("revision: close canonical series: %w", err))
		}
	}()
	opened, err := file.Stat()
	if err != nil {
		return fileSeries{}, true, fmt.Errorf("revision: inspect opened canonical series: %w", err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(entry, opened) {
		return fileSeries{}, true, fmt.Errorf("revision: canonical series %q changed during open", name)
	}
	current, err := root.Lstat(filename)
	if err != nil {
		return fileSeries{}, true, fmt.Errorf("revision: inspect canonical series after open: %w", err)
	}
	if !current.Mode().IsRegular() || !os.SameFile(current, opened) {
		return fileSeries{}, true, fmt.Errorf("revision: canonical series %q changed during open", name)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return fileSeries{}, true, fmt.Errorf("revision: read canonical series: %w", err)
	}
	if err := json.Unmarshal(data, &series); err != nil {
		return fileSeries{}, true, fmt.Errorf("revision: decode canonical series: %w", err)
	}
	if err := validateCanonicalSeries(name, series); err != nil {
		return fileSeries{}, true, err
	}
	return series, true, nil
}

func validateCanonicalSeries(name string, series fileSeries) error {
	invalid := fmt.Errorf("revision: invalid canonical series state for %q", name)
	if series.Status.Name != name || series.Status.RevisionCount == 0 || len(series.RevisionUIDs) == 0 || uint64(len(series.RevisionUIDs)) > series.Status.RevisionCount {
		return invalid
	}
	uids := make(map[string]bool, len(series.RevisionUIDs))
	for _, uid := range series.RevisionUIDs {
		if uid == "" || uids[uid] {
			return invalid
		}
		uids[uid] = true
	}
	if !uids[series.Status.DefaultRevisionUID] || !uids[series.Status.LatestRevisionUID] {
		return invalid
	}
	bound := make(map[string]bool, len(series.RevisionNames))
	for revisionName, uid := range series.RevisionNames {
		if revisionName == "" || !uids[uid] || bound[uid] {
			return invalid
		}
		bound[uid] = true
	}
	return nil
}
