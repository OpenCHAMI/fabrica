// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/annotations"
	"github.com/openchami/fabrica/pkg/codegen/testfixtures"
)

func TestGenerateEntAdapterDedicated(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("change to temp directory: %v", err)
	}

	gen := NewGenerator(tmpDir, "test", "github.com/test/project")
	gen.StorageType = "ent"
	gen.DBDriver = "postgres"

	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}

	type TokenSpec struct {
		Value       string `json:"value"`
		DisplayName string `json:"display_name"`
	}

	type Token struct {
		Spec TokenSpec
	}

	if err := gen.RegisterResource(&Token{}); err != nil {
		t.Fatalf("RegisterResource failed: %v", err)
	}

	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated

	valueAnnots := annotations.NewFieldAnnotations("Value")
	valueAnnots.Storage = &annotations.StorageConfig{
		Type: annotations.StorageTypeHashed,
		Hash: &annotations.HashConfig{
			Algorithm: annotations.HashAlgorithmBcrypt,
			Cost:      12,
		},
	}
	valueAnnots.Sensitive = true
	valueAnnots.Immutable = true
	annots.Fields["Value"] = valueAnnots

	nameAnnots := annotations.NewFieldAnnotations("DisplayName")
	nameAnnots.Index = &annotations.IndexConfig{
		Type: annotations.IndexTypeBTree,
	}
	annots.Fields["DisplayName"] = nameAnnots

	gen.SetResourceAnnotations("Token", annots)

	if err := gen.GenerateEntAdapter(); err != nil {
		t.Fatalf("GenerateEntAdapter failed: %v", err)
	}

	genericAdapter := filepath.Join("internal", "storage", "ent_adapter.go")
	if _, err := os.Stat(genericAdapter); os.IsNotExist(err) {
		t.Fatal("expected generic ent_adapter.go to exist")
	}

	dedicatedAdapter := filepath.Join("internal", "storage", "ent_adapter_token.go")
	if _, err := os.Stat(dedicatedAdapter); os.IsNotExist(err) {
		t.Fatalf("expected dedicated adapter %s to exist", dedicatedAdapter)
	}

	content, err := os.ReadFile(dedicatedAdapter)
	if err != nil {
		t.Fatalf("read adapter file: %v", err)
	}

	adapterStr := string(content)

	if !strings.Contains(adapterStr, "ToEntToken") {
		t.Error("expected ToEntToken function")
	}

	if !strings.Contains(adapterStr, "FromEntToken") {
		t.Error("expected FromEntToken function")
	}

	if !strings.Contains(adapterStr, "UpdateTokenFromResource") {
		t.Error("expected UpdateTokenFromResource function")
	}

	if !strings.Contains(adapterStr, "QueryTokenByName") {
		t.Error("expected QueryTokenByName function")
	}

	if !strings.Contains(adapterStr, "is immutable") {
		t.Error("expected immutability comment for Value field")
	}
}

func TestGeneratedDedicatedAdapterMappedTypesRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping generated adapter integration test in short mode")
	}

	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("change to temp directory: %v", err)
	}

	writeMappedTypesModule(t, origDir, tmpDir)

	gen := NewGenerator(tmpDir, "test", "github.com/openchami/fabrica")
	gen.StorageType = "ent"
	gen.DBDriver = "sqlite3"
	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}
	if err := gen.RegisterResource(&testfixtures.MappedToken{}); err != nil {
		t.Fatalf("RegisterResource failed: %v", err)
	}
	annots := annotations.NewResourceAnnotations()
	annots.IsResource = true
	annots.StorageMode = annotations.StorageModeDedicated
	annots.Fields["IssuedAt"] = annotations.NewFieldAnnotations("IssuedAt")
	annots.Fields["ConsumedAt"] = annotations.NewFieldAnnotations("ConsumedAt")
	annots.Fields["Scopes"] = annotations.NewFieldAnnotations("Scopes")
	annots.Fields["Fingerprint"] = annotations.NewFieldAnnotations("Fingerprint")
	annots.Fields["ReplayAttempts"] = annotations.NewFieldAnnotations("ReplayAttempts")
	annots.Fields["Labels"] = annotations.NewFieldAnnotations("Labels")
	gen.SetResourceAnnotations("MappedToken", annots)

	if err := gen.GenerateEntSchemas(); err != nil {
		t.Fatalf("GenerateEntSchemas failed: %v", err)
	}
	if err := gen.GenerateEntAdapter(); err != nil {
		t.Fatalf("GenerateEntAdapter failed: %v", err)
	}
	for _, path := range []string{
		filepath.Join(tmpDir, "internal", "storage", "ent_adapter.go"),
		filepath.Join(tmpDir, "internal", "storage", "generate.go"),
	} {
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove generic generated file %s: %v", path, err)
		}
	}

	runInDir(t, tmpDir, "go", "run", "-mod=mod", "entgo.io/ent/cmd/ent", "generate", "./internal/storage/ent/schema")
	runInDir(t, tmpDir, "go", "mod", "tidy")
	writeMappedTypesRoundTripTest(t, tmpDir)
	runInDir(t, tmpDir, "go", "test", "./internal/storage")
}

func writeMappedTypesModule(t *testing.T, repoRoot string, dir string) {
	t.Helper()
	repoRoot = filepath.Clean(filepath.Join(repoRoot, "..", ".."))
	goMod := []byte("module github.com/openchami/fabrica\n\ngo 1.26.6\n\nrequire (\n\tentgo.io/ent v0.14.5\n\tgithub.com/mattn/go-sqlite3 v1.14.32\n)\n")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), goMod, 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	copyDir(t, filepath.Join(repoRoot, "pkg", "resource"), filepath.Join(dir, "pkg", "resource"))
	copyDir(t, filepath.Join(repoRoot, "pkg", "codegen", "testfixtures"), filepath.Join(dir, "pkg", "codegen", "testfixtures"))
}

func writeMappedTypesRoundTripTest(t *testing.T, dir string) {
	t.Helper()
	testPath := filepath.Join(dir, "internal", "storage", "mapped_token_roundtrip_test.go")
	content := []byte(`package storage

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/openchami/fabrica/internal/storage/ent"
	"github.com/openchami/fabrica/internal/storage/ent/mappedtoken"
	"github.com/openchami/fabrica/pkg/codegen/testfixtures"
	"github.com/openchami/fabrica/pkg/resource"
	_ "github.com/mattn/go-sqlite3"
)

func TestGeneratedDedicatedAdapterMappedTypesRoundTrip(t *testing.T) {
	ctx := context.Background()
	client, err := ent.Open("sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	if err != nil {
		t.Fatalf("open ent client: %v", err)
	}
	defer client.Close()
	if err := client.Schema.Create(ctx); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	consumedAt := time.Date(2026, 8, 27, 10, 30, 0, 0, time.UTC)
	issuedAt := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	replayAttempts := []time.Time{issuedAt, consumedAt}
	token := &testfixtures.MappedToken{
		Resource: resource.Resource{
			APIVersion: "v1",
			Kind:       "MappedToken",
			Metadata: resource.Metadata{
				Name:            "mapped-token",
				UID:             "tok-1",
				CreatedAt:       issuedAt,
				UpdatedAt:       consumedAt,
			},
		},
		Spec: testfixtures.MappedTokenSpec{
			Subject:        "user-1",
			UsageCount:     7,
			Revoked:        true,
			SequenceNumber: 42,
			Weight:         3.5,
			TTL:            90 * time.Second,
			IssuedAt:       issuedAt,
			ConsumedAt:     &consumedAt,
			Scopes:         []string{"read", "write"},
			Fingerprint:    []byte{1, 2, 3, 4},
			ReplayAttempts: replayAttempts,
			Labels:         map[string]string{"env": "test"},
		},
	}

	created, err := ToEntMappedToken(ctx, client, token)
	if err != nil {
		t.Fatalf("ToEntMappedToken: %v", err)
	}
	entity, err := created.Save(ctx)
	if err != nil {
		t.Fatalf("save mapped token: %v", err)
	}
	if entity.TTL != int64(90*time.Second) {
		t.Fatalf("entity TTL = %d, want %d", entity.TTL, int64(90*time.Second))
	}

	got, err := FromEntMappedToken(ctx, entity)
	if err != nil {
		t.Fatalf("FromEntMappedToken: %v", err)
	}
	if got.Spec.TTL != token.Spec.TTL {
		t.Fatalf("roundtrip TTL = %v, want %v", got.Spec.TTL, token.Spec.TTL)
	}
	if got.Spec.ConsumedAt == nil || !got.Spec.ConsumedAt.Equal(consumedAt) {
		t.Fatalf("roundtrip ConsumedAt = %v, want %v", got.Spec.ConsumedAt, consumedAt)
	}
	if !reflect.DeepEqual(got.Spec.Scopes, token.Spec.Scopes) {
		t.Fatalf("roundtrip scopes = %#v, want %#v", got.Spec.Scopes, token.Spec.Scopes)
	}
	if !reflect.DeepEqual(got.Spec.Fingerprint, token.Spec.Fingerprint) {
		t.Fatalf("roundtrip fingerprint = %#v, want %#v", got.Spec.Fingerprint, token.Spec.Fingerprint)
	}
	if !reflect.DeepEqual(got.Spec.ReplayAttempts, token.Spec.ReplayAttempts) {
		t.Fatalf("roundtrip replay attempts = %#v, want %#v", got.Spec.ReplayAttempts, token.Spec.ReplayAttempts)
	}
	if !reflect.DeepEqual(got.Spec.Labels, token.Spec.Labels) {
		t.Fatalf("roundtrip labels = %#v, want %#v", got.Spec.Labels, token.Spec.Labels)
	}

	updatedConsumedAt := consumedAt.Add(time.Hour)
	token.Spec.TTL = 5 * time.Minute
	token.Spec.ConsumedAt = &updatedConsumedAt
	update := client.MappedToken.UpdateOneID(entity.ID)
	if err := UpdateMappedTokenFromResource(ctx, update, token); err != nil {
		t.Fatalf("UpdateMappedTokenFromResource: %v", err)
	}
	if _, err := update.Save(ctx); err != nil {
		t.Fatalf("save updated token: %v", err)
	}
	updated, err := client.MappedToken.Query().Where(mappedtoken.UIDEQ("tok-1")).Only(ctx)
	if err != nil {
		t.Fatalf("query updated token: %v", err)
	}
	if updated.TTL != int64(5*time.Minute) {
		t.Fatalf("updated TTL = %d, want %d", updated.TTL, int64(5*time.Minute))
	}
	if updated.ConsumedAt == nil || !updated.ConsumedAt.Equal(updatedConsumedAt) {
		t.Fatalf("updated ConsumedAt = %v, want %v", updated.ConsumedAt, updatedConsumedAt)
	}
}
`)
	if err := os.WriteFile(testPath, content, 0o644); err != nil {
		t.Fatalf("write roundtrip test: %v", err)
	}
}

func runInDir(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, output)
	}
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read dir %s: %v", src, err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("create dir %s: %v", dst, err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			copyDir(t, srcPath, dstPath)
			continue
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("read file %s: %v", srcPath, err)
		}
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			t.Fatalf("write file %s: %v", dstPath, err)
		}
	}
}

func TestGenerateEntAdapterGenericOnly(t *testing.T) {
	tmpDir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("change to temp directory: %v", err)
	}

	gen := NewGenerator(tmpDir, "test", "github.com/test/project")
	gen.StorageType = "ent"

	if err := gen.LoadTemplates(); err != nil {
		t.Fatalf("LoadTemplates failed: %v", err)
	}

	type DeviceSpec struct {
		Name string `json:"name"`
	}

	type Device struct {
		Spec DeviceSpec
	}

	if err := gen.RegisterResource(&Device{}); err != nil {
		t.Fatalf("RegisterResource failed: %v", err)
	}

	if err := gen.GenerateEntAdapter(); err != nil {
		t.Fatalf("GenerateEntAdapter failed: %v", err)
	}

	genericAdapter := filepath.Join("internal", "storage", "ent_adapter.go")
	if _, err := os.Stat(genericAdapter); os.IsNotExist(err) {
		t.Fatal("expected generic ent_adapter.go to exist")
	}

	dedicatedAdapter := filepath.Join("internal", "storage", "ent_adapter_device.go")
	if _, err := os.Stat(dedicatedAdapter); !os.IsNotExist(err) {
		t.Error("did not expect dedicated adapter for Device (no annotation)")
	}
}
