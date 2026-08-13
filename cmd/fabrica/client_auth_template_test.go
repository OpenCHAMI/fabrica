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

func TestTemplate_ClientLibrary_RedactsAuthorizationInDebugLogs(t *testing.T) {
	clientTmpl := mustReadFile(t, "pkg/codegen/templates/client/client.go.tmpl")

	checks := []struct {
		content string
		msg     string
	}{
		{"showToken  bool", "per-client token visibility state"},
		{"const tokenPrefixLen = 6", "token prefix length"},
		{"func RedactToken(token string, show bool) string", "token redaction helper with explicit visibility state"},
		{"func redactAuthHeaderValues(vals []string, show bool) []string", "Authorization value redaction helper with explicit visibility state"},
		{"func isAuthorizationHeader(key string) bool", "Authorization header detector"},
		{"http.CanonicalHeaderKey(key) == \"Authorization\"", "case-insensitive Authorization header comparison"},
		{"c.logger.Debug().Msgf(\"  %s: %s\", k, redactAuthHeaderValues(v, c.showToken))", "redacted Authorization header logging using per-client state"},
	}

	for _, check := range checks {
		if !strings.Contains(clientTmpl, check.content) {
			t.Errorf("client template must contain %s", check.msg)
		}
	}

	if strings.Contains(clientTmpl, "var ShowToken bool") {
		t.Error("client template must not use package-global token visibility state")
	}

	if got := strings.Count(clientTmpl, "c.logger.Debug().Msgf(\"  %s: %s\", k, redactAuthHeaderValues(v, c.showToken))"); got != 2 {
		t.Errorf("client template must redact Authorization values in request and response logs; got %d occurrences", got)
	}
}

func TestTemplate_ClientLibrary_ShowTokenChaining(t *testing.T) {
	clientTmpl := mustReadFile(t, "pkg/codegen/templates/client/client.go.tmpl")

	if !strings.Contains(clientTmpl, "func (c *Client) WithShowToken(show bool) *Client") {
		t.Fatal("client template must provide WithShowToken")
	}
	if !strings.Contains(clientTmpl, "showToken:   show,") {
		t.Fatal("WithShowToken must set token visibility on the returned client")
	}
	if got := strings.Count(clientTmpl, "showToken:   c.showToken,"); got != 2 {
		t.Errorf("WithVersion and WithBearerToken must preserve token visibility; got %d copies", got)
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

func TestTemplate_ClientCLI_ShowTokenFlagSupport(t *testing.T) {
	cliTmpl := mustReadFile(t, "pkg/codegen/templates/client/cmd.go.tmpl")

	checks := []struct {
		content string
		msg     string
	}{
		{"--show-token   Show the full bearer token in debug logs", "document --show-token"},
		{"BoolVar(&showToken, \"show-token\", false", "register --show-token as disabled by default"},
		{"BindPFlag(\"show-token\"", "bind --show-token to configuration"},
		{"c = c.WithShowToken(viper.GetBool(\"show-token\"))", "apply --show-token to the client instance"},
	}

	for _, check := range checks {
		if !strings.Contains(cliTmpl, check.content) {
			t.Errorf("client CLI template must %s", check.msg)
		}
	}
}

func TestTemplate_ClientLibrary_LoggerSupport(t *testing.T) {
	clientTmpl := mustReadFile(t, "pkg/codegen/templates/client/client.go.tmpl")

	// Check logger field exists in Client struct
	if !strings.Contains(clientTmpl, "logger     zerolog.Logger") {
		t.Fatalf("client template must define logger field in Client struct")
	}

	// Check NewClient signature accepts logger parameter
	if !strings.Contains(clientTmpl, "func NewClient(baseURL string, httpClient *http.Client, logger zerolog.Logger)") {
		t.Fatalf("client template NewClient must accept logger parameter")
	}

	// Check NewClientWithBearerToken signature accepts logger parameter
	if !strings.Contains(clientTmpl, "func NewClientWithBearerToken(baseURL, bearerToken string, httpClient *http.Client, logger zerolog.Logger)") {
		t.Fatalf("client template NewClientWithBearerToken must accept logger parameter")
	}

	// Check DefaultLogger function exists
	if !strings.Contains(clientTmpl, "func DefaultLogger() zerolog.Logger") {
		t.Fatalf("client template must provide DefaultLogger function")
	}

	// Check NewLogger function exists
	if !strings.Contains(clientTmpl, "func NewLogger(level LogLevel) (zerolog.Logger, error)") {
		t.Fatalf("client template must provide NewLogger function")
	}

	// Check debug logging in doRequest method
	if !strings.Contains(clientTmpl, "c.logger.Debug().Msgf(\"%s: %s\", method, u.String())") {
		t.Fatalf("client template must log request method and URL")
	}
	if !strings.Contains(clientTmpl, "c.logger.Debug().Msg(\"Request headers:\")") {
		t.Fatalf("client template must log request headers")
	}
	if !strings.Contains(clientTmpl, "c.logger.Debug().Msg(\"Request body:\")") {
		t.Fatalf("client template must log request body")
	}
	if !strings.Contains(clientTmpl, "c.logger.Debug().Msg(\"Response status: \" + resp.Status)") {
		t.Fatalf("client template must log response status")
	}
	if !strings.Contains(clientTmpl, "c.logger.Debug().Msg(\"Response headers:\")") {
		t.Fatalf("client template must log response headers")
	}
	if !strings.Contains(clientTmpl, "c.logger.Debug().Msg(\"Response body:\")") {
		t.Fatalf("client template must log response body")
	}

	// Check LogLevel type exists
	if !strings.Contains(clientTmpl, "type LogLevel string") {
		t.Fatalf("client template must define LogLevel type")
	}

	// Check LogLevel constants exist
	if !strings.Contains(clientTmpl, "LogLevelInfo    LogLevel = \"info\"") {
		t.Fatalf("client template must define LogLevelInfo constant")
	}
	if !strings.Contains(clientTmpl, "LogLevelWarning LogLevel = \"warning\"") {
		t.Fatalf("client template must define LogLevelWarning constant")
	}
	if !strings.Contains(clientTmpl, "LogLevelDebug   LogLevel = \"debug\"") {
		t.Fatalf("client template must define LogLevelDebug constant")
	}

	// Check LogLevel methods exist
	if !strings.Contains(clientTmpl, "func (ll LogLevel) String() string") {
		t.Fatalf("client template must provide LogLevel.String() method")
	}
	if !strings.Contains(clientTmpl, "func (ll *LogLevel) Set(v string) error") {
		t.Fatalf("client template must provide LogLevel.Set() method")
	}
	if !strings.Contains(clientTmpl, "func (ll LogLevel) Type() string") {
		t.Fatalf("client template must provide LogLevel.Type() method")
	}

	// Check CompletionLogLevel function exists
	if !strings.Contains(clientTmpl, "func CompletionLogLevel(cmd *cobra.Command, args []string, toComplete string)") {
		t.Fatalf("client template must provide CompletionLogLevel function for shell completion")
	}

	// Check required imports
	if !strings.Contains(clientTmpl, "\"os\"") {
		t.Fatalf("client template must import os package")
	}
	if !strings.Contains(clientTmpl, "\"github.com/rs/zerolog\"") {
		t.Fatalf("client template must import github.com/rs/zerolog")
	}
	if !strings.Contains(clientTmpl, "\"github.com/spf13/cobra\"") {
		t.Fatalf("client template must import github.com/spf13/cobra")
	}
}

func TestTemplate_ClientCLI_LogLevelFlagSupport(t *testing.T) {
	cliTmpl := mustReadFile(t, "pkg/codegen/templates/client/cmd.go.tmpl")

	// Check logLevel variable declaration
	if !strings.Contains(cliTmpl, "logLevel    client.LogLevel") {
		t.Fatalf("cli template must declare logLevel variable")
	}

	// Check --log-level flag registration with -l shorthand
	if !strings.Contains(cliTmpl, "VarP(&logLevel, \"log-level\", \"l\"") {
		t.Fatalf("cli template must register --log-level flag with -l shorthand")
	}

	// Check help text for log-level flag
	if !strings.Contains(cliTmpl, "set verbosity of logs") {
		t.Fatalf("cli template must document --log-level flag purpose")
	}

	// Check viper binding
	if !strings.Contains(cliTmpl, "BindPFlag(\"log-level\"") {
		t.Fatalf("cli template must bind log-level flag to viper configuration")
	}

	// Check shell completion registration
	if !strings.Contains(cliTmpl, "RegisterFlagCompletionFunc(\"log-level\", client.CompletionLogLevel)") {
		t.Fatalf("cli template must register shell completion for log-level flag")
	}

	// Check getClient creates logger
	if !strings.Contains(cliTmpl, "logger, _ := client.NewLogger(logLevel)") {
		t.Fatalf("cli template getClient must create logger with NewLogger")
	}

	// Check logger is passed to NewClient
	if !strings.Contains(cliTmpl, "c, err := client.NewClient(serverURL, nil, logger)") {
		t.Fatalf("cli template must pass logger to NewClient")
	}
}
