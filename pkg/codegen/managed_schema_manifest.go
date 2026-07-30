// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const transactionManifestVersion = 1
const transactionManifestName = ".fabrica-transaction.json"

type transactionRole string

const (
	transactionRoleStaging    transactionRole = "staging"
	transactionRoleBackup     transactionRole = "backup"
	transactionRoleQuarantine transactionRole = "quarantine"
)

type transactionManifest struct {
	Version       int             `json:"version"`
	CanonicalRoot string          `json:"canonical_schema_root"`
	TransactionID string          `json:"transaction_id"`
	Role          transactionRole `json:"role"`
	PID           int             `json:"owner_pid"`
	Token         string          `json:"owner_token"`
}

func writeTransactionManifest(ops managedSchemaOperations, artifact string, manifest transactionManifest) error {
	content, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal transaction manifest: %w", err)
	}
	content = append(content, '\n')
	path := filepath.Join(artifact, transactionManifestName)
	if err := ops.writeFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write transaction manifest %s: %w", path, err)
	}
	return nil
}

func readTransactionManifest(ops managedSchemaOperations, artifact string) (transactionManifest, error) {
	path := filepath.Join(artifact, transactionManifestName)
	info, err := ops.lstat(path)
	if err != nil {
		return transactionManifest{}, fmt.Errorf("inspect transaction manifest %s: %w", path, errors.Join(errUnmanagedSchemaPath, err))
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return transactionManifest{}, fmt.Errorf("protected transaction manifest %s: %w", path, errUnmanagedSchemaPath)
	}
	content, err := ops.readFile(path)
	if err != nil {
		return transactionManifest{}, fmt.Errorf("read transaction manifest %s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var manifest transactionManifest
	if err := decoder.Decode(&manifest); err != nil {
		return transactionManifest{}, fmt.Errorf("parse transaction manifest %s: %w", path, errUnmanagedSchemaPath)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return transactionManifest{}, fmt.Errorf("trailing transaction manifest data %s: %w", path, errUnmanagedSchemaPath)
	}
	return manifest, nil
}

func (o *managedSchemaOutput) expectedManifest(role transactionRole, transactionID string) transactionManifest {
	return transactionManifest{
		Version: transactionManifestVersion, CanonicalRoot: o.canonical,
		TransactionID: transactionID, Role: role, PID: o.pid, Token: o.token,
	}
}

func (o *managedSchemaOutput) validateArtifactShape(path string, role transactionRole, transactionID string) (transactionManifest, error) {
	info, err := o.ops.lstat(path)
	if err != nil {
		return transactionManifest{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return transactionManifest{}, fmt.Errorf("protected transaction artifact %s: %w", path, errUnmanagedSchemaPath)
	}
	manifest, err := readTransactionManifest(o.ops, path)
	if err != nil {
		return transactionManifest{}, err
	}
	if manifest.Version != transactionManifestVersion || manifest.CanonicalRoot != o.canonical ||
		manifest.Role != role || manifest.TransactionID == "" || manifest.Token == "" || manifest.PID <= 0 ||
		(transactionID != "" && manifest.TransactionID != transactionID) {
		return transactionManifest{}, fmt.Errorf("transaction manifest identity mismatch for %s: %w", path, errUnmanagedSchemaPath)
	}
	return manifest, nil
}

func (o *managedSchemaOutput) validateArtifact(path string, role transactionRole, transactionID string) (transactionManifest, error) {
	manifest, err := o.validateArtifactShape(path, role, transactionID)
	if err != nil {
		return transactionManifest{}, err
	}
	expected := o.expectedManifest(role, transactionID)
	if manifest != expected {
		return transactionManifest{}, fmt.Errorf("transaction owner mismatch for %s: %w", path, errUnmanagedSchemaPath)
	}
	return manifest, nil
}

func (o *managedSchemaOutput) validateRecoveryArtifact(path string, role transactionRole, transactionID string) (transactionManifest, error) {
	return o.validateArtifactShape(path, role, transactionID)
}

func (o *managedSchemaOutput) createArtifact(path string, role transactionRole) error {
	if err := o.ops.mkdir(path, 0o700); err != nil {
		return err
	}
	manifest := o.expectedManifest(role, o.transactionID)
	if err := writeTransactionManifest(o.ops, path, manifest); err != nil {
		return err
	}
	return nil
}

func (o *managedSchemaOutput) removeArtifact(path string, role transactionRole, transactionID string) error {
	if _, err := o.validateArtifact(path, role, transactionID); err != nil {
		return err
	}
	return o.ops.removeAll(path)
}

func (o *managedSchemaOutput) removeRecoveryArtifact(path string, expected transactionManifest) error {
	manifest, err := o.validateRecoveryArtifact(path, expected.Role, expected.TransactionID)
	if err != nil {
		return err
	}
	if manifest != expected {
		return fmt.Errorf("recovery owner changed for %s: %w", path, errUnmanagedSchemaPath)
	}
	return o.ops.removeAll(path)
}
