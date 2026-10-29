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
)

// Use the checkout CLI and user-owned resource inputs, then execute the actual
// generated handlers and loader. Fresh generation and regeneration must agree.
func TestGeneratedValidationPolicy(t *testing.T) {
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
	for _, scenario := range []struct {
		name    string
		modes   []string
		enabled bool
	}{
		{"fresh-strict", []string{"strict"}, true},
		{"fresh-warn", []string{"warn"}, true},
		{"fresh-disabled", []string{"disabled"}, true},
		{"regenerated", []string{"strict", "warn", "disabled", "strict"}, true},
		{"boolean-disables", []string{"strict", "warn"}, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			dir := filepath.Join(workspace, scenario.name)
			run(t, workspace, binary, "init", scenario.name, "--module", "example.com/validationfixture", "--storage-type", "file")
			run(t, dir, "go", "mod", "edit", "-replace", "github.com/openchami/fabrica="+repo)
			run(t, dir, binary, "add", "resource", "Node")
			paths, err := filepath.Glob(filepath.Join(dir, "apis", "*", "*", "node_types.go"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("resource paths: %v, %v", paths, err)
			}
			types, err := os.ReadFile(paths[0])
			if err != nil {
				t.Fatal(err)
			}
			// Replace the scaffold hook with a test application hook while keeping
			// tags and all other user-owned resource source intact.
			start := bytes.Index(types, []byte("func (r *Node) Validate("))
			if start < 0 {
				t.Fatal("missing scaffold validation hook")
			}
			end := bytes.Index(types[start:], []byte("\n}"))
			if end < 0 {
				t.Fatal("missing scaffold validation hook end")
			}
			hookMethod := bytes.Replace(types[start:start+end], []byte("return nil"), []byte("return validateNodePolicy(r)"), 1)
			types = bytes.Join([][]byte{types[:start], hookMethod, types[start+end:]}, nil)
			types = bytes.Replace(types, []byte(`json:"phase,omitempty" yaml:"phase,omitempty"`), []byte(`json:"phase,omitempty" yaml:"phase,omitempty" validate:"omitempty,oneof=Pending Ready"`), 1)
			if err := os.WriteFile(paths[0], types, 0644); err != nil {
				t.Fatal(err)
			}
			apiDir := filepath.Dir(paths[0])
			apiPackage := filepath.Base(apiDir)
			hook := strings.ReplaceAll(validationPolicyHook, "API_PACKAGE", apiPackage)
			hookPath := filepath.Join(apiDir, "validation.go")
			if err := os.WriteFile(hookPath, []byte(hook), 0644); err != nil {
				t.Fatal(err)
			}
			mainPath := filepath.Join(dir, "cmd/server/main.go")
			main, err := os.ReadFile(mainPath)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := configpkg.LoadConfig(dir)
			if err != nil {
				t.Fatal(err)
			}
			apiRelative, err := filepath.Rel(dir, apiDir)
			if err != nil {
				t.Fatal(err)
			}
			for _, mode := range scenario.modes {
				cfg.Features.Validation = configpkg.ValidationConfig{Enabled: scenario.enabled, Mode: mode}
				if err := configpkg.SaveConfig(dir, cfg); err != nil {
					t.Fatal(err)
				}
				run(t, dir, binary, "generate", "--fabrica-source", repo)
				run(t, dir, "go", "mod", "tidy")
				effective := mode
				if !scenario.enabled {
					effective = "disabled"
				}
				test := strings.NewReplacer("API_IMPORT", "example.com/validationfixture/"+filepath.ToSlash(apiRelative), "EXPECTED_MODE", effective).Replace(validationPolicyRuntimeTests)
				if err := os.WriteFile(filepath.Join(dir, "cmd/server/validation_policy_test.go"), []byte(test), 0644); err != nil {
					t.Fatal(err)
				}
				run(t, dir, "go", "test", "./cmd/server", "-run", "^TestValidationPolicyHTTP$", "-count=1")
				got, err := os.ReadFile(mainPath)
				if err != nil || !bytes.Equal(got, main) {
					t.Fatalf("main changed: %v", err)
				}
				got, err = os.ReadFile(hookPath)
				if err != nil || string(got) != hook {
					t.Fatalf("custom validator changed: %v", err)
				}
				_, err = os.Stat(filepath.Join(dir, "internal/middleware/validation_middleware_generated.go"))
				if (err == nil) != (effective != "disabled") {
					t.Fatalf("mode=%s middleware: %v", effective, err)
				}
			}
		})
	}
}

const validationPolicyHook = `package API_PACKAGE
import("errors";"sync/atomic")
var ValidationCalls atomic.Int64
func validateNodePolicy(n *Node)error{
 ValidationCalls.Add(1)
 if n.Spec.Description=="custom-reject" {return errors.New("custom validation rejected description")}
 return nil
}
`

const validationPolicyRuntimeTests = `package main
import(
 "bytes"
 "context"
 "encoding/json"
 "log"
 "net/http"
 "net/http/httptest"
 "testing"
 "strings"

 "github.com/go-chi/chi/v5"
 "github.com/spf13/viper"
 "github.com/openchami/fabrica/pkg/events"
 api "API_IMPORT"
)
type validationBus struct{count int}
func(*validationBus)Start(){}
func(b *validationBus)Publish(context.Context,events.Event)error{b.count++;return nil}
func(*validationBus)Subscribe(string,events.EventHandler)(events.SubscriptionID,error){return "",nil}
func(*validationBus)Unsubscribe(events.SubscriptionID)error{return nil}
func(*validationBus)Close()error{return nil}
func TestValidationPolicyHTTP(t *testing.T){
 const mode="EXPECTED_MODE"
 viper.Reset();viper.Set("data_dir",t.TempDir());config:=DefaultConfig();if err:=viper.Unmarshal(config);err!=nil{t.Fatal(err)}
 // Runtime settings cannot override the generation-time validation policy.
 viper.Set("validation_mode","disabled");viper.Set("validation_enabled",false)
 features,err:=loadGeneratedFeatures(context.Background(),config);if err!=nil{t.Fatal(err)};defer features.Close()
 if err:=registerResourcePrefixes();err!=nil{t.Fatal(err)}
 router:=chi.NewRouter();features.Use(router)
 router.Post("/nodes",CreateNode);router.Get("/nodes",GetNodes);router.Get("/nodes/{uid}",GetNode)
 router.Put("/nodes/{uid}",UpdateNode);router.Patch("/nodes/{uid}",PatchNode)
 router.Put("/nodes/{uid}/status",UpdateNodeStatus);router.Patch("/nodes/{uid}/status",PatchNodeStatus)
 bus:=&validationBus{};oldConfig,oldBus:=events.GetEventConfig(),events.GetGlobalEventBus();defer func(){events.SetEventConfig(oldConfig);events.SetGlobalEventBus(oldBus)}()
 eventConfig:=events.DefaultEventConfig();eventConfig.Enabled=true;eventConfig.LifecycleEventsEnabled=true;events.SetEventConfig(eventConfig);events.SetGlobalEventBus(bus)
 request:=func(method,path,body,etag string)*httptest.ResponseRecorder{
  req:=httptest.NewRequest(method,path,strings.NewReader(body));req.Header.Set("Content-Type","application/json");if strings.HasPrefix(strings.TrimSpace(body),"["){req.Header.Set("Content-Type","application/json-patch+json")};if etag!=""{req.Header.Set("If-Match",etag)}
  response:=httptest.NewRecorder();router.ServeHTTP(response,req);return response
 }
 node:=func(response *httptest.ResponseRecorder)api.Node{t.Helper();var n api.Node;if err:=json.Unmarshal(response.Body.Bytes(),&n);err!=nil{t.Fatalf("decode status %d: %s: %v",response.Code,response.Body,err)};return n}

 for _, body := range []string{
  "{}", "null",
  "{\"metadata\":{\"name\":\"missing-spec\"}}",
  "{\"spec\":{}}",
  "{\"metadata\":null,\"spec\":{}}",
  "{\"metadata\":{\"name\":\"null-spec\"},\"spec\":null}",
 } {
  before := request("GET", "/nodes", "", "")
  bus.count=0;api.ValidationCalls.Store(0)
  result := request("POST", "/nodes", body, "")
  if result.Code!=400 { t.Fatalf("mode=%s missing/null envelope %s: %d %s",mode,body,result.Code,result.Body) }
  if bus.count!=0 || api.ValidationCalls.Load()!=0 {t.Fatal("mandatory rejection published events or invoked custom validation")}
  after := request("GET", "/nodes", "", "")
  if before.Code!=200 || after.Code!=200 || !bytes.Equal(before.Body.Bytes(),after.Body.Bytes()) { t.Fatal("mandatory rejection changed storage") }
 }
 // Presence and nonzero values are distinct. Do not introduce the source
 // commit's separate ordinary-create name requirement in this port.
 for _, body := range []string{
  "{\"metadata\":{\"name\":\"empty-spec\"},\"spec\":{}}",
  "{\"metadata\":{},\"spec\":{}}",
 } {
  result:=request("POST","/nodes",body,"")
  if result.Code!=201 {t.Fatalf("mode=%s present zero-value envelope %s: %d %s",mode,body,result.Code,result.Body)}
 }
 for _,tc:=range []struct{name,method,suffix,body string;create bool}{
  {"create tags","POST","",` + "`" + `{"metadata":{"name":"invalid"},"spec":{"description":"` + "`" + `+strings.Repeat("x",201)+` + "`" + `"}}` + "`" + `,true},
  {"create hook","POST","",` + "`" + `{"metadata":{"name":"invalid"},"spec":{"description":"custom-reject"}}` + "`" + `,true},
  {"put tags","PUT","",` + "`" + `{"spec":{"description":"` + "`" + `+strings.Repeat("x",201)+` + "`" + `"}}` + "`" + `,false},
  {"put hook","PUT","",` + "`" + `{"spec":{"description":"custom-reject"}}` + "`" + `,false},
  {"patch tags","PATCH","",` + "`" + `{"description":"` + "`" + `+strings.Repeat("x",201)+` + "`" + `"}` + "`" + `,false},
  {"patch hook","PATCH","",` + "`" + `{"description":"custom-reject"}` + "`" + `,false},
  {"status put","PUT","/status",` + "`" + `{"phase":"Broken"}` + "`" + `,false},
  {"status patch","PATCH","/status",` + "`" + `{"phase":"Broken"}` + "`" + `,false},
 }{
  t.Run(tc.name,func(t *testing.T){
   created:=request("POST","/nodes",` + "`" + `{"metadata":{"name":"valid"},"spec":{"description":"initial"}}` + "`" + `,"");if created.Code!=201{t.Fatalf("valid create: %d %s",created.Code,created.Body)}
   initial:=node(created);path:="/nodes/"+initial.Metadata.UID
   seeded:=request("PUT",path+"/status",` + "`" + `{"phase":"Ready"}` + "`" + `,"*");if seeded.Code!=200{t.Fatalf("valid status: %d %s",seeded.Code,seeded.Body)}
   before:=request("GET",path,"","");if before.Code!=200{t.Fatalf("read baseline: %d %s",before.Code,before.Body)}
   initial=node(before)
   if tc.create{path="/nodes"}
   bus.count=0;api.ValidationCalls.Store(0)
   var logs bytes.Buffer;oldLog:=log.Writer();log.SetOutput(&logs);defer log.SetOutput(oldLog)
   result:=request(tc.method,path+tc.suffix,tc.body,"*")
   want:=200;if tc.create{want=201};if mode=="strict"{want=400}
   if result.Code!=want{t.Fatalf("mode=%s: %d want %d: %s",mode,result.Code,want,result.Body)}
   if mode=="disabled"&&(api.ValidationCalls.Load()!=0||logs.Len()!=0){t.Fatalf("disabled ran optional checks: calls=%d logs=%s",api.ValidationCalls.Load(),logs.String())}
   if mode=="warn"&&!strings.Contains(logs.String(),"WARN: validation failed"){t.Fatalf("missing warn log: %s",logs.String())}
   if mode=="strict"{
    if bus.count!=0{t.Fatalf("rejection emitted %d events",bus.count)}
    after:=request("GET","/nodes/"+initial.Metadata.UID,"","")
    if !bytes.Equal(before.Body.Bytes(),after.Body.Bytes()){t.Fatalf("rejection changed state: before=%s after=%s",before.Body,after.Body)}
    return
   }
   if bus.count==0{t.Fatal("successful mutation omitted event")}
   var saved api.Node
   if tc.create{saved=node(result);persisted:=request("GET","/nodes/"+saved.Metadata.UID,"","");saved=node(persisted)}else{saved=node(request("GET",path,"",""))}
   if tc.suffix=="/status"{
    if saved.Status.Phase!="Broken"||saved.Spec.Description!=initial.Spec.Description{t.Fatalf("status/spec persistence: %+v",saved)}
   }else{
    expected:="custom-reject";if strings.Contains(tc.name,"tags"){expected=strings.Repeat("x",201)}
    if saved.Spec.Description!=expected{t.Fatalf("spec was not persisted: %+v",saved)}
    if !tc.create&&saved.Status.Phase!=initial.Status.Phase{t.Fatalf("spec update changed status: %+v",saved)}
   }
  })
 }
 baseline:=request("POST","/nodes",` + "`" + `{"metadata":{"name":"mandatory"},"spec":{"description":"valid"}}` + "`" + `,"");n:=node(baseline);path:="/nodes/"+n.Metadata.UID
 for _,tc:=range []struct{method,path,body,etag string;want int}{
  {"POST","/nodes","{","",400},
  {"PUT",path,"{","*",400},
  {"PUT",path+"/status","{","*",400},
  {"PATCH",path,` + "`" + `[{"op":"remove","path":"/missing"}]` + "`" + `,"*",422},
  {"PATCH",path+"/status",` + "`" + `[{"op":"remove","path":"/missing"}]` + "`" + `,"*",422},
  {"PUT",path,` + "`" + `{"spec":{"description":"valid"}}` + "`" + `,"\"stale\"",412},
  {"PUT","/nodes/missing",` + "`" + `{"spec":{"description":"valid"}}` + "`" + `,"*",404},
 }{got:=request(tc.method,tc.path,tc.body,tc.etag);if got.Code!=tc.want{t.Fatalf("mandatory %s %s mode=%s: %d want %d: %s",tc.method,tc.path,mode,got.Code,tc.want,got.Body)}}
 missingUID:=httptest.NewRecorder();UpdateNode(missingUID,httptest.NewRequest("PUT","/nodes/",strings.NewReader("{}")));if missingUID.Code!=http.StatusBadRequest{t.Fatalf("missing UID = %d",missingUID.Code)}
}
`
