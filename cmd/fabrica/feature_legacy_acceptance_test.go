// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"text/template"
	"time"

	configpkg "github.com/openchami/fabrica/internal/config"
)

func acceptanceApplicationRoute(t *testing.T, main []byte) []byte {
	t.Helper()
	needle := []byte(`public.Get("/health", healthHandler)`)
	if !bytes.Contains(main, needle) {
		t.Fatal("main lacks public health registration")
	}
	main = bytes.Replace(main, []byte(`"net/http"`), []byte("\"net/http\"\n\"github.com/openchami/fabrica/pkg/events\""), 1)
	return bytes.Replace(main, needle, append(append([]byte{}, needle...), []byte(`
 public.Get("/application/features",func(w http.ResponseWriter,r *http.Request){ _,_ = fmt.Fprintf(w,"%t",events.GetGlobalEventBus()!=nil) })
 // Application customization must survive the explicit adoption.
 `)...), 1)
}

func (p *acceptanceProject) observeServer(t *testing.T, wantBus bool, wantMetrics int) {
	t.Helper()
	binary := filepath.Join(p.workspace, "acceptance-service")
	p.run(t, p.dir, true, nil, "go", "build", "-o", binary, "./cmd/server")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "serve", "--host", "127.0.0.1", "--port", strconv.Itoa(port), "--data-dir", filepath.Join(p.workspace, "legacy-data"))
	cmd.Dir = p.dir
	cmd.Env = append(os.Environ(), "TOKENSMITH_JWKS_URL=", "TOKENSMITH_AUTHZ_MODE=off")
	log, err := os.CreateTemp(p.workspace, "legacy-server-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = log.Close() }()
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	var processError error
	go func() { processError = cmd.Wait(); close(exited) }()
	defer func() {
		select {
		case <-exited:
		default:
			_ = cmd.Process.Signal(os.Interrupt)
			select {
			case <-exited:
			case <-time.After(5 * time.Second):
				cancel()
				<-exited
			}
		}
	}()
	client := &http.Client{Timeout: time.Second}
	base := "http://127.0.0.1:" + strconv.Itoa(port)
	deadline := time.Now().Add(10 * time.Second)
	for {
		select {
		case <-exited:
			data, _ := os.ReadFile(log.Name())
			t.Fatalf("service exited: %v\n%s", processError, data)
		default:
		}
		response, err := client.Get(base + "/health")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			data, _ := os.ReadFile(log.Name())
			t.Fatalf("service did not start: %v\n%s", err, data)
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, tc := range []struct {
		path   string
		status int
		body   string
	}{
		{"/application/features", 200, strconv.FormatBool(wantBus)},
		{"/metrics", wantMetrics, ""},
		{"/nodes/", 200, ""}, // Legacy AuthN selection remains init-time; adopted test turns it off explicitly.
		{"/service/status", 200, ""},
		{"/.well-known/api-catalog", 200, ""},
	} {
		response, err := client.Get(base + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		if response.StatusCode != tc.status || tc.body != "" && string(body) != tc.body {
			t.Fatalf("live %s: %d %s; want %d %s", tc.path, response.StatusCode, body, tc.status, tc.body)
		}
	}
}

func TestFeatureLoaderLegacyExplicitAdoption(t *testing.T) {
	h := newFeatureAcceptance(t)
	p := h.project(t, "legacy-service", "file")
	mainPath := filepath.Join(p.dir, "cmd/server/main.go")
	adopted := acceptanceApplicationRoute(t, p.owned[mainPath])
	source, err := os.ReadFile(filepath.Join("testdata", "acceptance", "legacy_main.go.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := template.New("legacy").Parse(string(source))
	if err != nil {
		t.Fatal(err)
	}
	var rendered bytes.Buffer
	legacyData := map[string]interface{}{
		"ModulePath": "example.com/acceptance", "ProjectName": "acceptance", "EnvPrefix": "ACCEPTANCE",
		"FabricaVersion": "legacy-fixture", "CopyrightYear": 2026, "GeneratedAt": "historical fixture",
		"Description": "legacy acceptance", "WithStorage": true, "StorageType": "file", "DBDriver": "sqlite3",
		"WithAuth": false, "WithEvents": false, "WithReconcile": false, "WithVersion": false,
	}
	if err := tmpl.Execute(&rendered, legacyData); err != nil {
		t.Fatal(err)
	}
	helperSource, err := os.ReadFile(filepath.Join("testdata", "acceptance", "legacy_runtime.go.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	helperTemplate, err := template.New("legacy-runtime").Parse(string(helperSource))
	if err != nil {
		t.Fatal(err)
	}
	var helper bytes.Buffer
	if err := helperTemplate.Execute(&helper, legacyData); err != nil {
		t.Fatal(err)
	}
	helperPath := filepath.Join(p.dir, "cmd/server/runtime_helpers_generated.go")
	if err := os.WriteFile(helperPath, helper.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	legacy := acceptanceApplicationRoute(t, rendered.Bytes())
	if err := os.WriteFile(mainPath, legacy, 0644); err != nil {
		t.Fatal(err)
	}
	p.owned[mainPath] = legacy
	// Reconstruct the pre-loader service boundary; generation remains responsible
	// for every subsequent generated output. No Git history is needed in CI.
	for _, name := range []string{"features_generated.go", "features_manifest_generated.json", ".fabrica-features.lock"} {
		if err := os.Remove(filepath.Join(p.dir, "cmd/server", name)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	p.generate(t)
	p.observeServer(t, false, 404)
	for _, metrics := range []bool{true, false} {
		p.cfg.Features.Metrics.Enabled = metrics
		p.cfg.Features.Events.Enabled = true
		p.cfg.Features.Auth.Enabled = true
		if err := configpkg.SaveConfig(p.dir, p.cfg); err != nil {
			t.Fatal(err)
		}
		p.generate(t)
		// Metrics retain their supported legacy helper behavior, while init-time
		// event/auth startup does not silently adopt the loader after YAML changes.
		status := 404
		if metrics {
			status = 200
		}
		p.observeServer(t, false, status)
		if _, err := os.Stat(filepath.Join(p.dir, "cmd/server/features_manifest_generated.json")); !os.IsNotExist(err) {
			t.Fatalf("legacy scaffold silently adopted the loader: %v", err)
		}
	}
	// Apply the documented one-time main migration, retaining application routes
	// and preserving unowned helpers outside the service before ownership begins.
	backup := filepath.Join(h.workspace, "legacy-preserved")
	for _, path := range []string{"cmd/server/runtime_helpers_generated.go", "cmd/server/auth_helpers_generated.go", "cmd/server/metrics_helpers_generated.go", "cmd/server/metrics_generated.go", "internal/middleware/validation_middleware_generated.go", "internal/middleware/conditional_middleware_generated.go", "internal/middleware/versioning_middleware_generated.go", "internal/middleware/event_bus_generated.go"} {
		from := filepath.Join(p.dir, path)
		if _, err := os.Lstat(from); os.IsNotExist(err) {
			continue
		} else if err != nil {
			t.Fatal(err)
		}
		to := filepath.Join(backup, path)
		if err := os.MkdirAll(filepath.Dir(to), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(from, to); err != nil {
			t.Fatal(err)
		}
	}
	preserved := acceptanceTree(t, backup)
	if err := os.WriteFile(mainPath, adopted, 0644); err != nil {
		t.Fatal(err)
	}
	p.owned[mainPath] = adopted
	p.cfg.Features.Auth.Enabled = false
	p.cfg.Features.Security.AuthN.Enabled = false
	if err := configpkg.SaveConfig(p.dir, p.cfg); err != nil {
		t.Fatal(err)
	}
	p.generate(t)
	p.observeServer(t, true, 404)
	if _, err := os.Stat(filepath.Join(p.dir, "cmd/server/features_manifest_generated.json")); err != nil {
		t.Fatal(err)
	}
	// After adoption, the same event feature can be turned off without main edits.
	p.cfg.Features.Events.Enabled = false
	if err := configpkg.SaveConfig(p.dir, p.cfg); err != nil {
		t.Fatal(err)
	}
	p.generate(t)
	p.observeServer(t, false, 404)
	if !reflect.DeepEqual(preserved, acceptanceTree(t, backup)) {
		t.Fatal("adoption changed preserved legacy customizations")
	}
	if !strings.Contains(string(p.owned[mainPath]), "Application customization") {
		t.Fatal("application route was not preserved")
	}
}
