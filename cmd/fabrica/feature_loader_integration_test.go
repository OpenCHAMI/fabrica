// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	configpkg "github.com/openchami/fabrica/internal/config"
	"github.com/openchami/fabrica/internal/constants"
)

// Exercise the checkout-built CLI and real generated services, rather than
// reproducing the feature loader in a hand-written fixture.
func TestFeatureLoaderGeneratedService(t *testing.T) {
	if testing.Short() {
		t.Skip("generated service integration")
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	binary := filepath.Join(workspace, "fabrica")
	run := func(t *testing.T, dir, executable string, args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, executable, args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", executable, args, err, out)
		}
		return out
	}
	run(t, repo, "go", "build", "-o", binary, "./cmd/fabrica")
	for _, backend := range []string{"file", "ent"} {
		t.Run(backend, func(t *testing.T) {
			dir := filepath.Join(workspace, backend)
			run(t, workspace, binary, "init", backend, "--module", "example.com/"+backend, "--storage-type", backend)
			run(t, dir, "go", "mod", "edit", "-replace", "github.com/openchami/fabrica="+repo,
				"-require", constants.TokenSmithModulePath+"@"+constants.TokenSmithVersion)
			run(t, dir, binary, "add", "resource", "Node")
			mainPath := filepath.Join(dir, "cmd/server/main.go")
			main, err := os.ReadFile(mainPath)
			if err != nil {
				t.Fatal(err)
			}
			main = append(main, []byte("\n// Application customization survives regeneration.\n")...)
			if err := os.WriteFile(mainPath, main, 0644); err != nil {
				t.Fatal(err)
			}
			cfg, err := configpkg.LoadConfig(dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, enabled := range []bool{false, true, false} {
				cfg.Features.Metrics.Enabled = enabled
				cfg.Features.Security.AuthN.Enabled = enabled
				cfg.Features.Auth.Enabled = false
				cfg.Features.Events.Enabled = enabled
				cfg.Features.Events.BusType = "memory"
				cfg.Features.Reconciliation.Enabled = enabled
				cfg.Features.Reconciliation.WorkerCount = 2
				cfg.Features.Reconciliation.RequeueDelay = 7
				if err := configpkg.SaveConfig(dir, cfg); err != nil {
					t.Fatal(err)
				}
				run(t, dir, binary, "generate", "--fabrica-source", repo)
				run(t, dir, "go", "mod", "tidy")
				test := strings.NewReplacer("METRICS_ENABLED", map[bool]string{true: "true", false: "false"}[enabled], "AUTH_IMPORTS", map[bool]string{true: featureLoaderAuthImports, false: ""}[enabled]).Replace(featureLoaderRuntimeTests)
				// The auth-specific test is compiled only when auth is generated.
				if enabled {
					test += featureLoaderAuthRuntimeTests
				}
				testPath := filepath.Join(dir, "cmd/server/features_test.go")
				if err := os.WriteFile(testPath, []byte(test), 0644); err != nil {
					t.Fatal(err)
				}
				run(t, dir, "go", "test", "./cmd/server", "-count=1")
				run(t, dir, "go", "build", "./...")
				got, _ := os.ReadFile(mainPath)
				if !bytes.Equal(got, main) {
					t.Fatal("main customization changed")
				}
				loaderPath := filepath.Join(dir, "cmd/server/features_generated.go")
				before, _ := os.ReadFile(loaderPath)
				run(t, dir, binary, "generate", "--fabrica-source", repo)
				after, _ := os.ReadFile(loaderPath)
				if !bytes.Equal(before, after) {
					t.Fatal("repeat generation rewrote loader")
				}
			}
			if backend == "file" {
				// A project may adopt an existing implementation as custom storage.
				generated := filepath.Join(dir, "internal/storage/storage_generated.go")
				data, err := os.ReadFile(generated)
				if err != nil {
					t.Fatal(err)
				}
				owned := filepath.Join(dir, "internal/storage/project_owned.go")
				if err := os.WriteFile(owned, data, 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(generated); err != nil {
					t.Fatal(err)
				}
				cfg.Features.Storage.Type = "custom"
				if err := configpkg.SaveConfig(dir, cfg); err != nil {
					t.Fatal(err)
				}
				run(t, dir, binary, "generate", "--fabrica-source", repo)
				run(t, dir, "go", "mod", "tidy")
				run(t, dir, "go", "test", "./cmd/server", "-count=1")
				run(t, dir, "go", "build", "./...")
				after, _ := os.ReadFile(owned)
				if !bytes.Equal(after, data) {
					t.Fatal("project storage was changed")
				}
			}
			if backend == "ent" {
				cfg.Features.Storage.Type = "file"
				if err := configpkg.SaveConfig(dir, cfg); err != nil {
					t.Fatal(err)
				}
				before := make(map[string][]byte)
				for _, path := range []string{
					"cmd/server/main.go", "cmd/server/features_generated.go",
					"cmd/server/features_manifest_generated.json",
					"pkg/resources/register_generated.go", "internal/storage/storage_generated.go",
				} {
					data, err := os.ReadFile(filepath.Join(dir, path))
					if err != nil {
						t.Fatal(err)
					}
					before[path] = data
				}
				cmd := exec.Command(binary, "generate", "--fabrica-source", repo)
				cmd.Dir = dir
				out, err := cmd.CombinedOutput()
				if err == nil || !strings.Contains(string(out), "Ent-to-file storage migration") {
					t.Fatalf("backend transition: %v\n%s", err, out)
				}
				for path, want := range before {
					got, err := os.ReadFile(filepath.Join(dir, path))
					if err != nil || !bytes.Equal(got, want) {
						t.Fatalf("rejected migration changed %s: %v", path, err)
					}
				}
				// Preserve the old backend outside the service before migrating.
				backup := t.TempDir()
				for _, path := range []string{
					"internal/storage/ent", "internal/storage/ent_adapter.go",
					"internal/storage/ent_queries_generated.go", "internal/storage/ent_transactions_generated.go",
					"internal/storage/generate.go", "cmd/server/export.go", "cmd/server/import.go",
				} {
					if err := os.Rename(filepath.Join(dir, path), filepath.Join(backup, filepath.Base(path))); err != nil {
						t.Fatal(err)
					}
				}
				run(t, dir, binary, "generate", "--fabrica-source", repo)
				run(t, dir, "go", "mod", "tidy")
				run(t, dir, "go", "test", "./cmd/server", "-count=1")
				run(t, dir, "go", "build", "./...")
			}
			// Changed feature settings must fail before registration or main is modified.
			cfg.Features.Metrics.Enabled = true
			if err := configpkg.SaveConfig(dir, cfg); err != nil {
				t.Fatal(err)
			}
			registration := filepath.Join(dir, "pkg/resources/register_generated.go")
			before, _ := os.ReadFile(registration)
			cmd := exec.Command(binary, "generate", "--handlers", "--fabrica-source", repo)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "run full 'fabrica generate'") {
				t.Fatalf("selective transition: %v\n%s", err, out)
			}
			after, _ := os.ReadFile(registration)
			if !bytes.Equal(before, after) {
				t.Fatal("rejected generation mutated registration")
			}
		})
	}
}

const featureLoaderRuntimeTests = `package main

import (
 AUTH_IMPORTS
 "context"
 "net/http"
 "net/http/httptest"
 "path/filepath"
 "testing"

 "github.com/go-chi/chi/v5"
 "github.com/spf13/viper"
)

func TestGeneratedFeatureRuntime(t *testing.T) {
 viper.Reset()
 config:=DefaultConfig()
 // Set storage through runtime config so the same test covers file and Ent.
 viper.Set("data_dir",t.TempDir())
 viper.Set("database_url","file:"+filepath.Join(t.TempDir(),"test.db")+"?_fk=1")
 if err:=viper.Unmarshal(config);err!=nil{t.Fatal(err)}
 // Authentication is intentionally disabled for this composition test.
 viper.Set("auth_enabled",false)
 if err:=viper.Unmarshal(config);err!=nil{t.Fatal(err)}
 features,err:=loadGeneratedFeatures(context.Background(),config)
 if err!=nil{t.Fatal(err)}
 defer features.Close()
 r:=chi.NewRouter()
 features.Use(r)
 r.Group(func(public chi.Router){
  public.Get("/health",healthHandler)
  public.Get("/openapi.json",ServeOpenAPISpec)
  public.Get("/docs",ServeSwaggerUI)
  RegisterGeneratedPublicRoutes(public)
  features.RegisterPublicRoutes(public)
 })
 r.Group(func(protected chi.Router){
  features.UseProtected(protected)
  RegisterGeneratedRoutes(protected)
  protected.Get("/application",func(w http.ResponseWriter,r *http.Request){w.WriteHeader(http.StatusOK)})
 })
 for _,path:=range []string{"/health","/openapi.json","/docs","/service/status","/application"}{
  w:=httptest.NewRecorder();r.ServeHTTP(w,httptest.NewRequest("GET",path,nil))
  if w.Code!=http.StatusOK{t.Errorf("%s: %d",path,w.Code)}
 }
 w:=httptest.NewRecorder();r.ServeHTTP(w,httptest.NewRequest("GET","/metrics",nil))
 want:=http.StatusNotFound;if METRICS_ENABLED{want=http.StatusOK}
 if w.Code!=want{t.Fatalf("metrics=%d, want %d",w.Code,want)}
 features.Close();features.Close() // cleanup is idempotent
}

func TestGeneratedFeatureCleanupOrder(t *testing.T){
 var order []int
 f:=&generatedFeatureSet{cleanup:[]func(){func(){order=append(order,1)},func(){order=append(order,2)},func(){order=append(order,3)}}}
 f.Close();f.Close()
 if len(order)!=3 || order[0]!=3 || order[1]!=2 || order[2]!=1{t.Fatalf("cleanup=%v",order)}
}
`

const featureLoaderAuthImports = `
 "crypto"
 "crypto/rand"
 "crypto/rsa"
 "crypto/sha256"
 "encoding/base64"
 "encoding/json"
 "math/big"
 "os"
 "time"
 "github.com/openchami/fabrica/pkg/events"
 "github.com/openchami/tokensmith/pkg/keys"
`

const featureLoaderAuthRuntimeTests = `
func TestGeneratedAuthRequiresConfiguration(t *testing.T){
 viper.Reset()
 config:=DefaultConfig()
 viper.Set("data_dir",t.TempDir())
 viper.Set("database_url","file:"+filepath.Join(t.TempDir(),"failure.db")+"?_fk=1")
 if err:=viper.Unmarshal(config);err!=nil{t.Fatal(err)}
 t.Setenv("TOKENSMITH_JWKS_URL","")
 features,err:=loadGeneratedFeatures(context.Background(),config)
 if err==nil{features.Close();t.Fatal("missing JWKS configuration accepted")}
 // The failed auth step must unwind the already started reconciliation/bus.
 if events.GetGlobalEventBus()!=nil{t.Fatal("event bus leaked after failed startup")}
 features.Close()
}

func TestGeneratedAuthRoutingAndModes(t *testing.T){
 key,err:=rsa.GenerateKey(rand.Reader,2048);if err!=nil{t.Fatal(err)}
 encode:=base64.RawURLEncoding.EncodeToString
 kid,err:=keys.RFC7638Thumbprint(&key.PublicKey);if err!=nil{t.Fatal(err)}
 jwks:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  w.Header().Set("Content-Type","application/json")
  json.NewEncoder(w).Encode(map[string]any{"keys":[]any{map[string]string{
   "kty":"RSA","alg":"RS256","use":"sig","kid":kid,
   "n":encode(key.N.Bytes()),"e":encode(big.NewInt(int64(key.E)).Bytes()),
  }}})
 }))
 defer jwks.Close()
 t.Setenv("TOKENSMITH_JWKS_URL",jwks.URL)
 for _,name:=range []string{"MODEL","POLICY","GROUPING"}{
  path,err:=filepath.Abs(filepath.Join("..","..","authz",map[string]string{"MODEL":"model.conf","POLICY":"policy.csv","GROUPING":"grouping.csv"}[name]))
  if err!=nil{t.Fatal(err)}
  if _,err:=os.Stat(path);err!=nil{t.Fatal(err)}
  t.Setenv("TOKENSMITH_CASBIN_"+name,path)
 }
 headerJSON,err:=json.Marshal(map[string]string{"alg":"RS256","kid":kid,"typ":"JWT"});if err!=nil{t.Fatal(err)}
 header:=encode(headerJSON)
 claims,err:=json.Marshal(map[string]any{
 "sub":"tester","iss":"fixture","aud":[]string{"fixture"},"exp":time.Now().Add(time.Hour).Unix(),"iat":time.Now().Unix(),"nbf":time.Now().Unix(),
 "auth_level":"IAL2","auth_factors":2,"auth_methods":[]string{"service","certificate"},"session_id":"fixture","session_exp":time.Now().Add(time.Hour).Unix(),"auth_events":[]string{"service_auth"},
 })
 if err!=nil{t.Fatal(err)}
 signing:=header+"."+encode(claims)
 digest:=sha256.Sum256([]byte(signing))
 signature,err:=rsa.SignPKCS1v15(rand.Reader,key,crypto.SHA256,digest[:]);if err!=nil{t.Fatal(err)}
 token:=signing+"."+encode(signature)
 for _,mode:=range []string{"off","shadow","enforce"}{
  t.Run(mode,func(t *testing.T){
   t.Setenv("TOKENSMITH_AUTHZ_MODE",mode)
   config:=DefaultConfig()
   config.ReconcileEnabled=false
   viper.Reset()
   viper.Set("data_dir",t.TempDir())
   viper.Set("database_url","file:"+filepath.Join(t.TempDir(),"auth.db")+"?_fk=1")
   if err:=viper.Unmarshal(config);err!=nil{t.Fatal(err)}
   features,err:=loadGeneratedFeatures(context.Background(),config);if err!=nil{t.Fatal(err)}
   defer features.Close()
   r:=chi.NewRouter()
   features.Use(r)
   r.Group(func(public chi.Router){
    public.Get("/health",healthHandler)
    public.Get("/openapi.json",ServeOpenAPISpec)
    public.Get("/docs",ServeSwaggerUI)
    RegisterGeneratedPublicRoutes(public)
    features.RegisterPublicRoutes(public)
   })
   r.Group(func(protected chi.Router){features.UseProtected(protected);protected.Get("/protected",func(w http.ResponseWriter,r *http.Request){w.WriteHeader(http.StatusOK)})})
   for _,tc:=range []struct{path,token string;want int}{
    {"/health","",http.StatusOK},
    {"/openapi.json","",http.StatusOK},
    {"/docs","",http.StatusOK},
    {"/service/status","",http.StatusOK},
    {"/metrics","",http.StatusOK},
    {"/protected","",http.StatusUnauthorized},
    {"/protected",token,map[bool]int{true:http.StatusForbidden,false:http.StatusOK}[mode=="enforce"]},
   }{
    req:=httptest.NewRequest("GET",tc.path,nil)
    if tc.token!=""{req.Header.Set("Authorization","Bearer "+tc.token)}
    w:=httptest.NewRecorder();r.ServeHTTP(w,req)
    if w.Code!=tc.want{t.Fatalf("%s: %d, want %d: %s",tc.path,w.Code,tc.want,w.Body.String())}
   }
  })
 }
}
`
