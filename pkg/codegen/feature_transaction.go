// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const featureJournalName = ".fabrica-features-transaction.json"
const featureLockName = ".fabrica-features.lock"

// Target bytes are staged inside the journal itself. One atomic replacement
// persists both intent and immutable staged data, avoiding dangling stage paths.
type featureTransaction struct {
	Format   int
	Config   FeatureConfig
	Files    []featureChange
	Manifest featureChange
}
type featureState struct {
	Exists bool
	Digest string
}
type featureChange struct {
	Path   string
	Before featureState
	After  featureState
	Data   []byte
	Mode   uint32
}

// UnmarshalJSON rejects missing state instead of decoding it as absence.
func (state *featureState) UnmarshalJSON(data []byte) error {
	var fields struct {
		Exists *bool
		Digest *string
	}
	if err := decodeFeatureJSON(data, &fields); err != nil {
		return err
	}
	if fields.Exists == nil || fields.Digest == nil {
		return fmt.Errorf("missing explicit feature state")
	}
	state.Exists, state.Digest = *fields.Exists, *fields.Digest
	return nil
}

// UnmarshalJSON requires explicit old/target states, permissions, and staged data.
func (change *featureChange) UnmarshalJSON(data []byte) error {
	var fields struct {
		Path          *string
		Before, After *featureState
		Data          json.RawMessage
		Mode          *uint32
	}
	if err := decodeFeatureJSON(data, &fields); err != nil {
		return err
	}
	if fields.Path == nil || fields.Before == nil || fields.After == nil || fields.Mode == nil || len(fields.Data) == 0 {
		return fmt.Errorf("incomplete feature change")
	}
	var staged []byte
	if err := json.Unmarshal(fields.Data, &staged); err != nil {
		return err
	}
	*change = featureChange{Path: *fields.Path, Before: *fields.Before, After: *fields.After, Data: staged, Mode: *fields.Mode}
	return nil
}

// FeatureRecovery describes an interrupted transaction without modifying files.
// Paths are relative to the service root, and Config is the captured selection,
// which is recovered before a new selection is generated.
type FeatureRecovery struct {
	Config   FeatureConfig
	Writes   []string
	Removals []string
}

func (g *Generator) featureRoot() (*os.Root, error) {
	return os.OpenRoot(filepath.Clean(filepath.Join(g.OutputDir, "..", "..")))
}

func (g *Generator) featureRelative(path string) (string, error) {
	root, err := filepath.Abs(filepath.Join(g.OutputDir, "..", ".."))
	if err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, absolute)
	if err != nil {
		return "", err
	}
	if !filepath.IsLocal(rel) {
		return "", fmt.Errorf("feature path outside service: %s", path)
	}
	return filepath.ToSlash(rel), nil
}

func (g *Generator) featureJournalPath() (string, error) {
	return g.featureRelative(filepath.Join(g.OutputDir, featureJournalName))
}

// Reject symlinks and special files even inside the root: they must not redirect
// an owned output onto application code. os.Root also confines filesystem access.
func checkFeaturePath(root *os.Root, path string) error {
	if !filepath.IsLocal(path) || filepath.ToSlash(filepath.Clean(path)) != path {
		return fmt.Errorf("invalid feature path %q", path)
	}
	parts := strings.Split(path, "/")
	for i := range parts {
		info, err := root.Lstat(filepath.FromSlash(strings.Join(parts[:i+1], "/")))
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || i < len(parts)-1 && !info.IsDir() || i == len(parts)-1 && !info.Mode().IsRegular() {
			return fmt.Errorf("feature path %s is not a regular file or has an unsafe parent", path)
		}
	}
	return nil
}

func readFeatureState(root *os.Root, path string) (featureState, []byte, error) {
	if err := checkFeaturePath(root, path); err != nil {
		return featureState{}, nil, err
	}
	data, err := root.ReadFile(filepath.FromSlash(path))
	if os.IsNotExist(err) {
		return featureState{}, nil, nil
	}
	if err != nil {
		return featureState{}, nil, err
	}
	return featureState{Exists: true, Digest: featureDigest(data)}, data, nil
}

func decodeFeatureJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("unexpected trailing recovery metadata: %v", err)
	}
	return nil
}

func (g *Generator) readFeatureTransaction(root *os.Root) (*featureTransaction, error) {
	path, err := g.featureJournalPath()
	if err != nil {
		return nil, err
	}
	if err := checkFeaturePath(root, path); err != nil {
		return nil, err
	}
	f, err := root.Open(filepath.FromSlash(path))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, (16<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 16<<20 {
		return nil, fmt.Errorf("feature recovery journal exceeds 16 MiB")
	}
	var tx featureTransaction
	if err := decodeFeatureJSON(data, &tx); err != nil {
		return nil, fmt.Errorf("invalid feature recovery journal: %w", err)
	}
	if err := g.validateFeatureTransaction(&tx); err != nil {
		return nil, fmt.Errorf("invalid feature recovery journal: %w", err)
	}
	return &tx, nil
}

func validFeatureState(state featureState) bool {
	if !state.Exists {
		return state.Digest == ""
	}
	if len(state.Digest) != 64 {
		return false
	}
	for _, c := range state.Digest {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

func (g *Generator) validateFeatureTransaction(tx *featureTransaction) error {
	if tx.Format != 1 {
		return fmt.Errorf("unsupported format %d", tx.Format)
	}
	if err := tx.Config.Validate(); err != nil {
		return err
	}
	if tx.Config != tx.Config.Normalize() {
		return fmt.Errorf("captured configuration is not normalized")
	}

	allowed := make(map[string]bool)
	selected := make(map[string]bool)
	captured := NewGenerator(g.OutputDir, g.PackageName, g.ModulePath)
	captured.SetFeatures(tx.Config)
	for _, output := range captured.featureOutputs() {
		path, err := g.featureRelative(output.path)
		if err != nil {
			return err
		}
		allowed[path] = true
		selected[path] = output.enabled
	}
	if len(tx.Files) != len(allowed) {
		return fmt.Errorf("incomplete owned-output list")
	}
	for _, change := range tx.Files {
		if !allowed[change.Path] {
			return fmt.Errorf("unknown or duplicate owned path %q", change.Path)
		}
		if change.After.Exists != selected[change.Path] {
			return fmt.Errorf("captured selection disagrees with %s", change.Path)
		}
		delete(allowed, change.Path)
	}
	manifestPath, err := g.featureRelative(filepath.Join(g.OutputDir, featureManifestName))
	if err != nil {
		return err
	}
	if tx.Manifest.Path != manifestPath || !tx.Manifest.After.Exists {
		return fmt.Errorf("invalid manifest path or state")
	}
	for _, change := range append(append([]featureChange{}, tx.Files...), tx.Manifest) {
		if !validFeatureState(change.Before) || !validFeatureState(change.After) || change.Mode & ^uint32(0777) != 0 {
			return fmt.Errorf("invalid recorded state for %s", change.Path)
		}
		if change.After.Exists {
			if featureDigest(change.Data) != change.After.Digest {
				return fmt.Errorf("staged data digest mismatch for %s", change.Path)
			}
		} else if len(change.Data) != 0 {
			return fmt.Errorf("staged deletion contains data for %s", change.Path)
		}
	}
	var manifest featureManifest
	if err := decodeFeatureJSON(tx.Manifest.Data, &manifest); err != nil {
		return err
	}
	if manifest.Format != 1 || manifest.Config != tx.Config {
		return fmt.Errorf("captured manifest/config mismatch")
	}
	expected := make(map[string]string)
	for _, change := range tx.Files {
		if !change.After.Exists {
			continue
		}
		path, err := filepath.Rel(filepath.Dir(filepath.FromSlash(manifestPath)), filepath.FromSlash(change.Path))
		if err != nil {
			return err
		}
		expected[filepath.ToSlash(path)] = change.After.Digest
	}
	if len(expected) != len(manifest.Files) {
		return fmt.Errorf("captured manifest file list mismatch")
	}
	for path, digest := range expected {
		if manifest.Files[path] != digest {
			return fmt.Errorf("captured manifest digest mismatch for %s", path)
		}
	}
	return nil
}

func (g *Generator) checkFeatureTransactionStates(root *os.Root, tx *featureTransaction) error {
	for _, change := range append(append([]featureChange{}, tx.Files...), tx.Manifest) {
		current, _, err := readFeatureState(root, change.Path)
		if err != nil {
			return err
		}
		if current != change.Before && current != change.After {
			return fmt.Errorf("feature recovery conflict: %s matches neither recorded state; preserve customizations and restore a recorded version before full 'fabrica generate'; recovery journal retained", change.Path)
		}
	}
	return nil
}

// InspectFeatureRecovery is read-only, including when recovery is needed or
// metadata is corrupt. It does not create a lock file, rewrite files, or clean up.
func (g *Generator) InspectFeatureRecovery() (*FeatureRecovery, error) {
	root, err := g.featureRoot()
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	tx, err := g.readFeatureTransaction(root)
	if err != nil || tx == nil {
		return nil, err
	}
	info := &FeatureRecovery{Config: tx.Config}
	for _, change := range tx.Files {
		if change.After.Exists {
			info.Writes = append(info.Writes, change.Path)
		} else if change.Before.Exists {
			info.Removals = append(info.Removals, change.Path)
		}
	}
	info.Writes = append(info.Writes, tx.Manifest.Path)
	sort.Strings(info.Writes)
	sort.Strings(info.Removals)
	if err := g.checkFeatureTransactionStates(root, tx); err != nil {
		return info, err
	}
	return info, nil
}

// PrepareFeatureGeneration is the mutating full-generation preflight. Selective
// requests and previews never recover; full requests finish the captured intent
// before checking ownership or planning their newly requested configuration.
func (g *Generator) PrepareFeatureGeneration(full bool) error {
	info, err := g.InspectFeatureRecovery()
	if err != nil {
		return err
	}
	if info != nil {
		if !full {
			return fmt.Errorf("feature recovery pending; run full 'fabrica generate' before selective generation")
		}
		if err := g.withFeatureLock(func(root *os.Root) error { return g.recoverFeatures(root) }); err != nil {
			return err
		}
	}
	return g.CheckFeatureGeneration(full)
}

func (g *Generator) withFeatureLock(run func(*os.Root) error) error {
	root, err := g.featureRoot()
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	path, err := g.featureRelative(filepath.Join(g.OutputDir, featureLockName))
	if err != nil {
		return err
	}
	if err := checkFeaturePath(root, path); err != nil {
		return err
	}
	file, err := root.OpenFile(filepath.FromSlash(path), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	// Keep the lock inode permanently; removing it would allow competing locks.
	if err := lockFeatureFile(file); err != nil {
		return fmt.Errorf("lock feature generation: %w", err)
	}
	defer unlockFeatureFile(file)
	return run(root)
}

func (g *Generator) featureOperation(operation, path string) error {
	if g.featureTransactionHook != nil {
		return g.featureTransactionHook(operation, path)
	}
	return nil
}

func (g *Generator) syncFeatureParents(root *os.Root, path string) error {
	for dir := filepath.Dir(filepath.FromSlash(path)); ; dir = filepath.Dir(dir) {
		if err := g.featureOperation("before-directory-sync", filepath.ToSlash(dir)); err != nil {
			return err
		}
		f, err := root.Open(dir)
		if os.IsNotExist(err) && dir != "." {
			continue
		}
		if err != nil {
			return err
		}
		err = syncFeatureDirectory(f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if err := g.featureOperation("after-directory-sync", filepath.ToSlash(dir)); err != nil {
			return err
		}
		if dir == "." {
			return nil
		}
	}
}

func featureTemporaryPath(path string) string { return path + ".fabrica-tmp" }

func (g *Generator) atomicFeatureWrite(root *os.Root, path string, data []byte, mode uint32) error {
	if err := checkFeaturePath(root, path); err != nil {
		return err
	}
	if err := root.MkdirAll(filepath.Dir(filepath.FromSlash(path)), 0755); err != nil {
		return err
	}
	temporary := featureTemporaryPath(path)
	if err := checkFeaturePath(root, temporary); err != nil {
		return err
	}
	if err := root.Remove(filepath.FromSlash(temporary)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := g.featureOperation("before-write", path); err != nil {
		return err
	}
	f, err := root.OpenFile(filepath.FromSlash(temporary), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close(); _ = root.Remove(filepath.FromSlash(temporary)) }()
	var n int
	if g.featureTransactionWrite != nil {
		n, err = g.featureTransactionWrite(f, data)
	} else {
		n, err = f.Write(data)
	}
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	if err := f.Chmod(os.FileMode(mode)); err != nil {
		return err
	}
	if err := g.featureOperation("after-write", path); err != nil {
		return err
	}
	if err := g.featureOperation("before-file-sync", path); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := g.featureOperation("after-file-sync", path); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := g.featureOperation("before-rename", path); err != nil {
		return err
	}
	if err := root.Rename(filepath.FromSlash(temporary), filepath.FromSlash(path)); err != nil {
		return err
	}
	if err := g.featureOperation("after-rename", path); err != nil {
		return err
	}
	return g.syncFeatureParents(root, path)
}

func (g *Generator) removeFeatureFile(root *os.Root, path string) error {
	if err := checkFeaturePath(root, path); err != nil {
		return err
	}
	if err := g.featureOperation("before-remove", path); err != nil {
		return err
	}
	if err := root.Remove(filepath.FromSlash(path)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := g.featureOperation("after-remove", path); err != nil {
		return err
	}
	return g.syncFeatureParents(root, path)
}

func (g *Generator) recoverFeatures(root *os.Root) error {
	tx, err := g.readFeatureTransaction(root)
	if err != nil {
		return err
	}
	if tx == nil {
		// A process may stop before publishing intent. No live writes happened;
		// discard that private staging even when the next selection is a no-op.
		journal, err := g.featureJournalPath()
		if err != nil {
			return err
		}
		return g.removeFeatureTemporary(root, journal)
	}
	// A prior process may have stopped after journal rename but before syncing
	// its directory. Make that intent durable before any recovery mutations.
	journal, err := g.featureJournalPath()
	if err != nil {
		return err
	}
	if err := g.syncFeatureParents(root, journal); err != nil {
		return err
	}
	return g.commitFeatureTransaction(root, tx)
}

func (g *Generator) commitFeatureTransaction(root *os.Root, tx *featureTransaction) error {
	if err := g.checkFeatureTransactionStates(root, tx); err != nil {
		return err
	}
	for _, change := range append(append([]featureChange{}, tx.Files...), tx.Manifest) {
		if change.Path == tx.Manifest.Path {
			for _, output := range tx.Files {
				state, _, err := readFeatureState(root, output.Path)
				if err != nil {
					return err
				}
				if state != output.After {
					return fmt.Errorf("feature recovery conflict at %s before manifest commit; recovery journal retained", output.Path)
				}
			}
		}
		current, _, err := readFeatureState(root, change.Path)
		if err != nil {
			return err
		}
		if current == change.After {
			// Re-observing a rename/removal does not prove its parent directory was
			// synced before interruption. Flush it before committing/cleaning intent.
			if change.Before != change.After {
				if err := g.syncFeatureParents(root, change.Path); err != nil {
					return err
				}
			}
			continue
		}
		if current != change.Before {
			return fmt.Errorf("feature recovery conflict at %s; recovery journal retained", change.Path)
		}
		if change.After.Exists {
			err = g.atomicFeatureWrite(root, change.Path, change.Data, change.Mode)
		} else {
			err = g.removeFeatureFile(root, change.Path)
		}
		if err != nil {
			return fmt.Errorf("feature transaction interrupted at %s: %w; run full 'fabrica generate' to recover", change.Path, err)
		}
	}
	// Remove private staging files before removing the journal, so interrupted
	// cleanup still has the durable intent needed for an idempotent retry.
	journal, err := g.featureJournalPath()
	if err != nil {
		return err
	}
	for _, path := range appendFeaturePaths(tx, journal) {
		if err := g.removeFeatureTemporary(root, path); err != nil {
			return err
		}
	}
	return g.removeFeatureFile(root, journal)
}

func (g *Generator) removeFeatureTemporary(root *os.Root, path string) error {
	temporary := featureTemporaryPath(path)
	if err := checkFeaturePath(root, temporary); err != nil {
		return err
	}
	if _, err := root.Lstat(filepath.FromSlash(temporary)); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return g.removeFeatureFile(root, temporary)
}

func appendFeaturePaths(tx *featureTransaction, journal string) []string {
	paths := []string{journal, tx.Manifest.Path}
	for _, change := range tx.Files {
		paths = append(paths, change.Path)
	}
	return paths
}

func (g *Generator) writeFeatureTransaction(root *os.Root, outputs []featureOutput, rendered map[string][]byte) error {
	manifestPath, err := g.featureRelative(filepath.Join(g.OutputDir, featureManifestName))
	if err != nil {
		return err
	}
	oldManifestState, oldManifestData, err := readFeatureState(root, manifestPath)
	if err != nil {
		return err
	}
	var previous featureManifest
	if oldManifestState.Exists {
		if err := json.Unmarshal(oldManifestData, &previous); err != nil {
			return err
		}
	}
	tx := &featureTransaction{Format: 1, Config: g.Config.Features.Normalize()}
	manifest := featureManifest{Format: 1, Config: tx.Config, Files: make(map[string]string)}
	for _, output := range outputs {
		path, err := g.featureRelative(output.path)
		if err != nil {
			return err
		}
		before, existing, err := readFeatureState(root, path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(g.OutputDir, output.path)
		if err != nil {
			return err
		}
		if before.Exists && previous.Files[filepath.ToSlash(relative)] != before.Digest {
			return fmt.Errorf("feature output %s is untracked or modified before staging", path)
		}
		change := featureChange{Path: path, Before: before, Mode: 0644}
		if before.Exists {
			info, err := root.Stat(filepath.FromSlash(path))
			if err != nil {
				return err
			}
			change.Mode = uint32(info.Mode().Perm())
		}
		if output.enabled {
			data := applyGeneratedFileIdentity(rendered[output.path], generatedFileIdentity(g.Version, g.Commit))
			if before.Exists && bytes.Equal(normalizeGeneratedTimestamps(existing), normalizeGeneratedTimestamps(data)) {
				data = existing
			}
			change.Data = data
			change.After = featureState{Exists: true, Digest: featureDigest(data)}
			manifest.Files[filepath.ToSlash(relative)] = change.After.Digest
		}
		tx.Files = append(tx.Files, change)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path, err := g.featureRelative(filepath.Join(g.OutputDir, featureManifestName))
	if err != nil {
		return err
	}
	before, _, err := readFeatureState(root, path)
	if err != nil {
		return err
	}
	tx.Manifest = featureChange{Path: path, Before: oldManifestState, After: featureState{Exists: true, Digest: featureDigest(data)}, Data: data, Mode: 0644}
	if oldManifestState.Exists {
		info, err := root.Stat(filepath.FromSlash(path))
		if err != nil {
			return err
		}
		tx.Manifest.Mode = uint32(info.Mode().Perm())
	}

	if before != oldManifestState {
		return fmt.Errorf("feature manifest changed before staging")
	}
	changed := before != tx.Manifest.After
	for _, change := range tx.Files {
		changed = changed || change.Before != change.After
	}
	if !changed {
		return nil
	}
	if err := g.validateFeatureTransaction(tx); err != nil {
		return err
	}
	journal, err := g.featureJournalPath()
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	if len(encoded) > 16<<20 {
		return fmt.Errorf("feature recovery journal exceeds 16 MiB")
	}
	if err := g.atomicFeatureWrite(root, journal, encoded, 0600); err != nil {
		return fmt.Errorf("persist feature recovery journal: %w", err)
	}
	return g.commitFeatureTransaction(root, tx)
}
