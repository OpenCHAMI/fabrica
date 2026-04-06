// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

func TestTemplate_ClientLibrary_BearerTokenSupport(t *testing.T) {
	clientTmpl := mustReadFile(t, "pkg/codegen/templates/client/client.go.tmpl")

	if !strings.Contains(clientTmpl, "bearerToken string") {
		t.Fatalf("client template must define bearerToken field")
	}
	if !strings.Contains(clientTmpl, "NewClientWithBearerToken") {
		t.Fatalf("client template must provide NewClientWithBearerToken helper")
	}
	if !strings.Contains(clientTmpl, "WithBearerToken") {
		t.Fatalf("client template must provide WithBearerToken helper")
	}
	if !strings.Contains(clientTmpl, "req.Header.Set(\"Authorization\", fmt.Sprintf(\"Bearer %s\", c.bearerToken))") {
		t.Fatalf("client template must set Authorization bearer header")
	}
}

func TestTemplate_ClientCLI_BearerTokenFlagSupport(t *testing.T) {
	cliTmpl := mustReadFile(t, "pkg/codegen/templates/client/cmd.go.tmpl")

	if !strings.Contains(cliTmpl, "--token        JWT bearer token") {
		t.Fatalf("cli template must document --token flag")
	}
	if !strings.Contains(cliTmpl, "StringVar(&bearerToken, \"token\"") {
		t.Fatalf("cli template must add --token flag")
	}
	if !strings.Contains(cliTmpl, "BindPFlag(\"token\"") {
		t.Fatalf("cli template must bind token flag to configuration")
	}
	if !strings.Contains(cliTmpl, "viper.GetString(\"token\")") {
		t.Fatalf("cli template must read token from viper/env/config")
	}
	if !strings.Contains(cliTmpl, "c = c.WithBearerToken(token)") {
		t.Fatalf("cli template must configure client with bearer token")
	}
}
