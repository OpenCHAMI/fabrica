// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

func TestTemplate_InitMain_WiresTokenSmithAuthZ(t *testing.T) {
	authHelperTmpl := mustReadFile(t, "pkg/codegen/templates/init/auth_helpers.go.tmpl")

	if !strings.Contains(authHelperTmpl, "{{.TokenSmithModulePath}}/pkg/authz") {
		t.Fatalf("init/auth_helpers.go.tmpl must import TokenSmith pkg/authz")
	}
	if !strings.Contains(authHelperTmpl, "{{.TokenSmithModulePath}}/pkg/authz/engine") {
		t.Fatalf("init/auth_helpers.go.tmpl must import TokenSmith authz engine")
	}
	if !strings.Contains(authHelperTmpl, "tokensmithauthz.NewMiddleware(") {
		t.Fatalf("init/auth_helpers.go.tmpl must construct TokenSmith authz middleware")
	}
	if !strings.Contains(authHelperTmpl, "authzRouteMapper{}") {
		t.Fatalf("init/auth_helpers.go.tmpl must use the generated authz route mapper")
	}
	if !strings.Contains(authHelperTmpl, "TOKENSMITH_CASBIN_GROUPING") {
		t.Fatalf("init/auth_helpers.go.tmpl must read TOKENSMITH_CASBIN_GROUPING")
	}
	if !strings.Contains(authHelperTmpl, "parseAuthZMode") {
		t.Fatalf("init/auth_helpers.go.tmpl must parse TOKENSMITH_AUTHZ_MODE")
	}
	if !strings.Contains(authHelperTmpl, "tokensmithauthz.WithOnDecision(logAuthZDecision)") {
		t.Fatalf("init/auth_helpers.go.tmpl must install the default authz decision hook")
	}
	if !strings.Contains(authHelperTmpl, "func logAuthZDecision(_ context.Context, rec tokensmithauthz.DecisionRecord)") {
		t.Fatalf("init/auth_helpers.go.tmpl must define a default authz decision logger")
	}
	if !strings.Contains(authHelperTmpl, "authzMiddleware = tokensmithauthz.NewMiddleware(") {
		t.Fatalf("init/auth_helpers.go.tmpl must produce an authz middleware handler")
	}
}

func TestTemplate_InitEnv_AdvertisesStarterAuthZFiles(t *testing.T) {
	envTmpl := mustReadFile(t, "pkg/codegen/templates/init/env.tmpl")

	if !strings.Contains(envTmpl, "TOKENSMITH_AUTHZ_MODE=off") {
		t.Fatalf("env template should default authz mode to off")
	}
	if !strings.Contains(envTmpl, "TOKENSMITH_CASBIN_GROUPING=./authz/grouping.csv") {
		t.Fatalf("env template should include grouping policy path")
	}
	if !strings.Contains(envTmpl, "Generated starter files live under ./authz by default") {
		t.Fatalf("env template should document starter authz files")
	}
}

func TestTemplate_StarterCasbinFilesExist(t *testing.T) {
	model := mustReadFile(t, "pkg/codegen/templates/authz/model.conf.tmpl")
	if !strings.Contains(model, "g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act") {
		t.Fatalf("starter model should match route object and action exactly with role inheritance")
	}

	policy := mustReadFile(t, "pkg/codegen/templates/authz/policy.csv.tmpl")
	if !strings.Contains(policy, "role:viewer") || !strings.Contains(policy, "role:editor") || !strings.Contains(policy, "role:admin") {
		t.Fatalf("starter policy should include viewer/editor/admin roles")
	}
	if !strings.Contains(policy, "{{range .Resources}}") {
		t.Fatalf("starter policy should render per-resource tuples")
	}

	grouping := mustReadFile(t, "pkg/codegen/templates/authz/grouping.csv.tmpl")
	if !strings.Contains(grouping, "g, role:editor, role:viewer") {
		t.Fatalf("starter grouping should include editor -> viewer inheritance")
	}
	if !strings.Contains(grouping, "g, role:admin, role:editor") {
		t.Fatalf("starter grouping should include admin -> editor inheritance")
	}
}
