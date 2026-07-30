// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
)

var errUnmanagedSchemaPath = errors.New("schema path is not Fabrica-managed")

type managedSchemaFile struct {
	name         string
	templateName string
	data         interface{}
}

type managedSchemaOperations struct {
	render       func(string, interface{}) ([]byte, error)
	format       func([]byte) ([]byte, error)
	lstat        func(string) (fs.FileInfo, error)
	mkdir        func(string, fs.FileMode) error
	mkdirAll     func(string, fs.FileMode) error
	readDir      func(string) ([]os.DirEntry, error)
	readFile     func(string) ([]byte, error)
	writeFile    func(string, []byte, fs.FileMode) error
	readlink     func(string) (string, error)
	symlink      func(string, string) error
	rename       func(string, string) error
	removeAll    func(string) error
	abs          func(string) (string, error)
	evalSymlinks func(string) (string, error)
	newID        func() string
	pid          func() int
	newFileLock  func(string) managedSchemaFileLock
}

func newManagedSchemaOperations(render func(string, interface{}) ([]byte, error)) managedSchemaOperations {
	return managedSchemaOperations{
		render: render, format: format.Source,
		lstat: os.Lstat, mkdir: os.Mkdir, mkdirAll: os.MkdirAll, readDir: os.ReadDir,
		readFile: os.ReadFile, writeFile: os.WriteFile,
		readlink: os.Readlink, symlink: os.Symlink,
		rename: os.Rename, removeAll: os.RemoveAll,
		abs: filepath.Abs, evalSymlinks: filepath.EvalSymlinks,
		newID: newManagedSchemaID, pid: os.Getpid, newFileLock: newManagedSchemaKernelLock,
	}
}

type managedSchemaOutput struct {
	current            string
	canonical          string
	staging            string
	backup             string
	quarantineArtifact string
	lock               string
	transactionID      string
	token              string
	pid                int
	ops                managedSchemaOperations
}

func newManagedSchemaOutput(current string, ops managedSchemaOperations) *managedSchemaOutput {
	abs, err := ops.abs(current)
	if err == nil {
		current = abs
	}
	output := &managedSchemaOutput{
		current: current, transactionID: ops.newID(), token: ops.newID(), pid: ops.pid(), ops: ops,
	}
	output.setArtifactPaths(current)
	return output
}

func (o *managedSchemaOutput) setArtifactPaths(root string) {
	o.current = root
	o.staging = root + ".fabrica-staging." + o.transactionID
	o.backup = root + ".fabrica-backup." + o.transactionID
	o.quarantineArtifact = root + ".fabrica-quarantine." + o.transactionID
	o.lock = root + ".fabrica.lock"
}

func (o *managedSchemaOutput) commit(files []managedSchemaFile) error {
	return o.withTransactionLock(func() error {
		if err := o.recoverLocked(); err != nil {
			return fmt.Errorf("recover managed schema output: %w", err)
		}
		return o.commitLocked(files)
	})
}

func (o *managedSchemaOutput) commitLocked(files []managedSchemaFile) error {
	if err := o.createArtifact(o.staging, transactionRoleStaging); err != nil {
		return fmt.Errorf("create managed schema staging artifact: %w", err)
	}
	if err := o.ops.mkdirAll(o.stagingTree(), 0o755); err != nil {
		return o.abortStaging(fmt.Errorf("create managed schema staging tree: %w", err))
	}
	if err := o.copyUnmanagedTree(); err != nil {
		return o.abortStaging(fmt.Errorf("preserve unmanaged schemas: %w", err))
	}
	for _, file := range files {
		if err := o.renderFile(file); err != nil {
			return o.abortStaging(err)
		}
	}
	return o.swapLocked()
}

func (o *managedSchemaOutput) renderFile(file managedSchemaFile) error {
	if file.name != filepath.Base(file.name) || filepath.Ext(file.name) != ".go" {
		return fmt.Errorf("invalid managed schema filename %q", file.name)
	}
	path := filepath.Join(o.stagingTree(), file.name)
	if _, err := o.ops.lstat(path); err == nil {
		return fmt.Errorf("managed schema target %s conflicts with preserved path: %w", path, errUnmanagedSchemaPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect managed schema target %s: %w", path, err)
	}
	if o.ops.render == nil {
		return errors.New("managed schema renderer is unavailable")
	}
	content, err := o.ops.render(file.templateName, file.data)
	if err != nil {
		return fmt.Errorf("render managed schema %s: %w", file.name, err)
	}
	formatted, err := o.ops.format(content)
	if err != nil {
		return fmt.Errorf("format managed schema %s: %w", file.name, err)
	}
	formatted, err = canonicalManagedSchemaHeader(formatted)
	if err != nil {
		return fmt.Errorf("canonicalize managed schema header %s: %w", file.name, err)
	}
	if err := o.ops.writeFile(path, formatted, 0o644); err != nil {
		return fmt.Errorf("write managed schema %s: %w", file.name, err)
	}
	return nil
}

func (o *managedSchemaOutput) abortStaging(cause error) error {
	if err := o.removeArtifact(o.staging, transactionRoleStaging, o.transactionID); err != nil {
		return errors.Join(cause, fmt.Errorf("remove failed schema staging directory: %w", err))
	}
	return cause
}

func (o *managedSchemaOutput) stagingTree() string {
	return filepath.Join(o.staging, "tree")
}

func (o *managedSchemaOutput) backupTree() string {
	return filepath.Join(o.backup, "tree")
}

func isFabricaManagedSchema(content []byte) bool {
	const marker = "// Code generated by Fabrica. DO NOT EDIT."
	line := content
	if index := bytes.IndexByte(content, '\n'); index >= 0 {
		line = content[:index]
	}
	return string(line) == marker
}

func canonicalManagedSchemaHeader(content []byte) ([]byte, error) {
	const marker = "// Code generated by Fabrica. DO NOT EDIT."
	index := bytes.IndexByte(content, '\n')
	if index < 0 {
		return nil, errors.New("generated schema has no header line")
	}
	line := string(content[:index])
	if line != marker && !strings.HasPrefix(line, "// Code generated by Fabrica ") {
		return nil, fmt.Errorf("generated schema header %q is not recognized", line)
	}
	result := make([]byte, 0, len(marker)+len(content)-index)
	result = append(result, marker...)
	result = append(result, content[index:]...)
	return result, nil
}

var managedSchemaIDFallback atomic.Uint64

func newManagedSchemaID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return strconv.Itoa(os.Getpid()) + "-" + strconv.FormatUint(managedSchemaIDFallback.Add(1), 10)
}
