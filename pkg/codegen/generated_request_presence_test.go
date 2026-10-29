// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGeneratedRequestEnvelopePresence(t *testing.T) {
	dir := t.TempDir()
	apiDir := filepath.Join(dir, "apis", "v1")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	apiSource := `package v1
type NodeSpec struct { Marker int ` + "`json:\"marker,omitempty\"`" + ` }
type NodeStatus struct{}
type Node struct{}
`
	if err := os.WriteFile(filepath.Join(apiDir, "types.go"), []byte(apiSource), 0o644); err != nil {
		t.Fatal(err)
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	goMod := fmt.Sprintf("module example.com/presence\n\ngo 1.26.10\n\nrequire github.com/openchami/fabrica v0.0.0\nreplace github.com/openchami/fabrica => %s\n", repoRoot)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(dir, "main", "example.com/presence")
	gen.Resources = []ResourceMetadata{{
		Name: "Node", PluralName: "nodes", Package: "example.com/presence/apis/v1", PackageAlias: "v1",
		TypeName: "*v1.Node", SpecType: "v1.NodeSpec", StatusType: "v1.NodeStatus", StorageName: "Node",
	}}
	if err := gen.LoadTemplates(); err != nil {
		t.Fatal(err)
	}
	if err := gen.GenerateModels(); err != nil {
		t.Fatal(err)
	}
	testSource := `package main
import (
 "encoding/json"
 "testing"
 "github.com/openchami/fabrica/pkg/validation"
)
func TestPresence(t *testing.T) {
 for _, body := range []string{"{}", "null", "{\"metadata\":null,\"spec\":null}", "{\"metadata\":{\"name\":\"provided\"}}", "{\"spec\":{}}", "{\"metadata\":null,\"spec\":{}}", "{\"metadata\":{},\"spec\":null}", "{\"metadata\":{},\"spec\":{\"marker\":1},\"spec\":null}", "{\"metadata\":{\"name\":\"dup\"},\"spec\":{},\"Metadata\":null}"} {
  var request CreateNodeRequest
  if err := json.Unmarshal([]byte(body), &request); err != nil { t.Fatal(err) }
  if err := validation.ValidateRequiredFields(&request); err == nil { t.Fatalf("accepted absent envelope: %s", body) }
 }
 // Presence follows the decoder's case-insensitive key matching.
 for _, body := range []string{"{\"metadata\":{},\"spec\":{}}", "{\"Metadata\":{},\"Spec\":{}}", "{\"METADATA\":{\"name\":\"upper\"},\"spec\":{\"marker\":1}}"} {
  var request CreateNodeRequest
  if err := json.Unmarshal([]byte(body), &request); err != nil { t.Fatal(err) }
  if err := validation.ValidateRequiredFields(&request); err != nil { t.Fatalf("rejected present envelope %s: %v", body, err) }
 }
 var constructed CreateNodeRequest
 constructed.Metadata.Name, constructed.Spec.Marker = "go", 1
 if err := validation.ValidateRequiredFields(&constructed); err != nil { t.Fatalf("rejected nonzero request built in Go: %v", err) }
 var request CreateNodeRequest
 if err := json.Unmarshal([]byte("{\"metadata\":{\"name\":\"first\"},\"spec\":{\"marker\":1}}"), &request); err != nil { t.Fatal(err) }
 if err := validation.ValidateRequiredFields(&request); err != nil { t.Fatal(err) }
 if err := json.Unmarshal([]byte("{\"metadata\":{\"name\":\"second\"}}"), &request); err != nil { t.Fatal(err) }
 if err := validation.ValidateRequiredFields(&request); err == nil { t.Fatal("reused request retained stale spec presence") }
}
`
	if err := os.WriteFile(filepath.Join(dir, "presence_test.go"), []byte(testSource), 0o644); err != nil {
		t.Fatal(err)
	}
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	tidy.Env = append(os.Environ(), "GOTOOLCHAIN=go1.26.10")
	if output, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("tidy generated request fixture: %v\n%s", err, output)
	}
	cmd := exec.Command("go", "test", "-run", "^TestPresence$", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=go1.26.10")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated request presence: %v\n%s", err, output)
	}
}
