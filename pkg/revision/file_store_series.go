// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package revision

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func (s *FileStore[T]) seriesPath(name string) string {
	return filepath.Join(s.root, "series", fmt.Sprintf("sha256.%x.json", sha256.Sum256([]byte(name))))
}

func (s *FileStore[T]) readSeries(name string) (fileSeries, error) {
	series, present, err := s.readCanonicalSeries(name)
	if err != nil {
		return fileSeries{}, err
	}
	if !present {
		legacy := base64.RawURLEncoding.EncodeToString([]byte(name)) + ".json"
		err = readJSONFile(filepath.Join(s.root, "series", legacy), &series)
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENAMETOOLONG) {
			return fileSeries{}, ErrSeriesNotFound
		}
		if err == nil && series.Status.Name != name {
			return fileSeries{}, ErrSeriesNotFound
		}
	}
	if err != nil {
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
