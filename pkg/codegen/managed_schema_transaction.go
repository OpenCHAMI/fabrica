// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type discoveredTransactionArtifact struct {
	path          string
	role          transactionRole
	transactionID string
	manifest      transactionManifest
}

func (o *managedSchemaOutput) recover() error {
	return o.withTransactionLock(o.recoverLocked)
}

func (o *managedSchemaOutput) recoverLocked() error {
	if err := o.validateCurrentRoot(); err != nil {
		if _, statErr := o.ops.lstat(o.current); !os.IsNotExist(statErr) {
			return err
		}
	}
	artifacts, err := o.discoverTransactionArtifacts()
	if err != nil {
		return err
	}
	backups := make([]discoveredTransactionArtifact, 0, 1)
	for _, artifact := range artifacts {
		if artifact.role == transactionRoleBackup {
			backups = append(backups, artifact)
		}
	}
	currentExists, err := schemaPathExists(o.ops.lstat, o.current)
	if err != nil {
		return fmt.Errorf("inspect current schema root during recovery: %w", err)
	}
	if !currentExists && len(backups) > 1 {
		return fmt.Errorf("multiple schema backups require manual resolution: %w", errUnmanagedSchemaPath)
	}
	if !currentExists && len(backups) == 1 {
		if err := o.restoreBackup(backups[0]); err != nil {
			return err
		}
	}
	for _, artifact := range artifacts {
		if artifact.role == transactionRoleBackup && !currentExists && len(backups) == 1 {
			continue
		}
		if err := o.removeRecoveryArtifact(artifact.path, artifact.manifest); err != nil {
			return fmt.Errorf("remove recovered transaction artifact %s: %w", artifact.path, err)
		}
	}
	return nil
}

func (o *managedSchemaOutput) discoverTransactionArtifacts() ([]discoveredTransactionArtifact, error) {
	entries, err := o.ops.readDir(filepath.Dir(o.current))
	if err != nil {
		return nil, fmt.Errorf("read schema transaction parent: %w", err)
	}
	base := filepath.Base(o.current)
	prefixes := []struct {
		prefix string
		role   transactionRole
	}{
		{prefix: base + ".fabrica-staging.", role: transactionRoleStaging},
		{prefix: base + ".fabrica-backup.", role: transactionRoleBackup},
		{prefix: base + ".fabrica-quarantine.", role: transactionRoleQuarantine},
	}
	artifacts := make([]discoveredTransactionArtifact, 0)
	for _, entry := range entries {
		for _, candidate := range prefixes {
			roleBase := strings.TrimSuffix(candidate.prefix, ".")
			if strings.HasPrefix(entry.Name(), roleBase) && !strings.HasPrefix(entry.Name(), candidate.prefix) {
				path := filepath.Join(filepath.Dir(o.current), entry.Name())
				return nil, fmt.Errorf("ambiguous legacy transaction artifact %s: %w", path, errUnmanagedSchemaPath)
			}
			if !strings.HasPrefix(entry.Name(), candidate.prefix) {
				continue
			}
			transactionID := strings.TrimPrefix(entry.Name(), candidate.prefix)
			path := filepath.Join(filepath.Dir(o.current), entry.Name())
			manifest, err := o.validateRecoveryArtifact(path, candidate.role, transactionID)
			if err != nil {
				return nil, fmt.Errorf("ambiguous transaction artifact %s: %w", path, err)
			}
			artifacts = append(artifacts, discoveredTransactionArtifact{
				path: path, role: candidate.role, transactionID: transactionID, manifest: manifest,
			})
			break
		}
	}
	return artifacts, nil
}

func (o *managedSchemaOutput) restoreBackup(artifact discoveredTransactionArtifact) error {
	tree := filepath.Join(artifact.path, "tree")
	info, err := o.ops.lstat(tree)
	if err != nil {
		return fmt.Errorf("inspect backup tree %s: %w", tree, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("backup tree %s is protected: %w", tree, errUnmanagedSchemaPath)
	}
	if err := o.ops.rename(tree, o.current); err != nil {
		return fmt.Errorf("restore backup tree %s: %w", tree, err)
	}
	if err := o.removeRecoveryArtifact(artifact.path, artifact.manifest); err != nil {
		return fmt.Errorf("remove restored backup wrapper %s: %w", artifact.path, err)
	}
	return nil
}

func (o *managedSchemaOutput) swapLocked() error {
	if _, err := o.validateArtifact(o.staging, transactionRoleStaging, o.transactionID); err != nil {
		return fmt.Errorf("validate staged schema artifact before swap: %w", err)
	}
	if err := o.validateCurrentRoot(); err != nil {
		if _, statErr := o.ops.lstat(o.current); !os.IsNotExist(statErr) {
			return o.abortStaging(err)
		}
	}
	currentExists, err := schemaPathExists(o.ops.lstat, o.current)
	if err != nil {
		return o.abortStaging(fmt.Errorf("inspect current schema tree before swap: %w", err))
	}
	if !currentExists {
		if err := o.ops.rename(o.stagingTree(), o.current); err != nil {
			return o.abortStaging(fmt.Errorf("install staged schema tree: %w", err))
		}
		return o.removeArtifact(o.staging, transactionRoleStaging, o.transactionID)
	}
	if err := o.createArtifact(o.backup, transactionRoleBackup); err != nil {
		return o.abortStaging(fmt.Errorf("create schema backup artifact: %w", err))
	}
	if err := o.ops.rename(o.current, o.backupTree()); err != nil {
		return errors.Join(o.abortStaging(fmt.Errorf("backup current schema tree: %w", err)), o.removeArtifact(o.backup, transactionRoleBackup, o.transactionID))
	}
	if err := o.ops.rename(o.stagingTree(), o.current); err != nil {
		installErr := fmt.Errorf("install staged schema tree: %w", err)
		if _, validateErr := o.validateArtifact(o.backup, transactionRoleBackup, o.transactionID); validateErr != nil {
			return errors.Join(installErr, fmt.Errorf("validate schema backup before rollback: %w", validateErr))
		}
		if rollbackErr := o.ops.rename(o.backupTree(), o.current); rollbackErr != nil {
			return errors.Join(installErr, fmt.Errorf("rollback current schema tree: %w", rollbackErr))
		}
		return errors.Join(o.abortStaging(installErr), o.removeArtifact(o.backup, transactionRoleBackup, o.transactionID))
	}
	return errors.Join(
		o.removeArtifact(o.staging, transactionRoleStaging, o.transactionID),
		o.removeArtifact(o.backup, transactionRoleBackup, o.transactionID),
	)
}

func (o *managedSchemaOutput) quarantine(name string) error {
	return o.withTransactionLock(func() error {
		if err := o.recoverLocked(); err != nil {
			return err
		}
		return o.quarantineLocked(name)
	})
}

func (o *managedSchemaOutput) quarantineLocked(name string) error {
	if name != filepath.Base(name) || filepath.Ext(name) != ".go" {
		return fmt.Errorf("invalid schema quarantine filename %q", name)
	}
	if err := o.validateCurrentRoot(); err != nil {
		return err
	}
	path := filepath.Join(o.current, name)
	info, err := o.ops.lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect schema quarantine path %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("refuse to quarantine protected schema path %s: %w", path, errUnmanagedSchemaPath)
	}
	content, err := o.ops.readFile(path)
	if err != nil {
		return fmt.Errorf("read schema quarantine path %s: %w", path, err)
	}
	if !isFabricaManagedSchema(content) {
		return fmt.Errorf("refuse to quarantine unmanaged schema path %s: %w", path, errUnmanagedSchemaPath)
	}
	if err := o.createArtifact(o.quarantineArtifact, transactionRoleQuarantine); err != nil {
		return fmt.Errorf("create schema quarantine artifact: %w", err)
	}
	if _, err := o.validateArtifact(o.quarantineArtifact, transactionRoleQuarantine, o.transactionID); err != nil {
		return fmt.Errorf("validate schema quarantine artifact: %w", err)
	}
	if err := o.ops.rename(path, filepath.Join(o.quarantineArtifact, name)); err != nil {
		return errors.Join(fmt.Errorf("quarantine stale managed schema %s: %w", path, err), o.removeArtifact(o.quarantineArtifact, transactionRoleQuarantine, o.transactionID))
	}
	return nil
}
