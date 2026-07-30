// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package annotations

import (
	"fmt"
	"strings"
	"testing"
)

type documentationContract struct {
	documents   map[string]markdownDocument
	testSymbols map[string]struct{}
}

type capabilityRow struct {
	capability string
	postgres   string
	sqlite     string
	evidence   string
}

type unsupportedRow struct {
	capability string
	contract   string
	evidence   string
}

var requiredCapabilities = []capabilityRow{
	{"Fields: `string`, `bool`, `int`, `int64`, `float64`, `time.Time`, and `[]string`", "Supported", "Supported", "TestCapabilities_supports_closed_field_matrix"},
	{"Nillable pointers: `*string`, `*bool`, `*int`, `*int64`, `*float64`, `*time.Time`", "Supported", "Supported", "TestCapabilities_supports_ent_nillable_scalar_pointers"},
	{"Bcrypt on `string` and `*string`", "Supported", "Supported", "TestCapabilities_supports_bcrypt"},
	{"Field directives require dedicated Ent storage", "Fail closed", "Fail closed", "TestPrepareResourceAnnotations_rejects_field_directives_when_storage_cannot_enforce_them"},
	{"Dedicated mode with file storage", "Rejected", "Rejected", "TestPrepareResourceAnnotations_rejects_dedicated_mode_for_file_backend"},
	{"File-backed resource version snapshots", "File backend only", "File backend only", "TestGeneratedFileVersioning_builds_and_runs_snapshot_runtime"},
	{"Ent resource version snapshots", "Rejected before output", "Rejected before output", "TestPrepareResourceAnnotations_rejects_ent_version_snapshots_for_every_storage_mode"},
	{"Required bcrypt create and omitted update", "Supported", "Supported", "TestDedicatedSecurity_generated_adapter_runtime"},
	{"Sensitive zero-value update semantics", "Supported", "Supported", "TestDedicatedSecurity_generated_adapter_runtime"},
	{"Persisted redacted write responses", "Supported", "Supported", "TestDedicatedSecurity_generated_adapter_runtime"},
	{"Scalar defaults: string, bool, int, int64, float64", "Supported", "Supported", "TestGeneratedDedicatedSchema_default_modifiers_match_pointer_shape"},
	{"Unique constraints", "Supported", "Supported", "TestGeneratedDedicatedIndex_baseline_portable_btree_and_unique"},
	{"Portable B-tree indexes", "Supported", "Supported", "TestGeneratedDedicatedIndex_baseline_portable_btree_and_unique"},
	{"GIN index on `[]string`", "Supported", "Rejected", "TestGeneratedDedicatedIndex_postgresql_methods_use_ent_annotations"},
	{"Hash index on scalar fields", "Supported", "Rejected", "TestGeneratedDedicatedIndex_postgresql_methods_use_ent_annotations"},
	{"Complete resource envelope", "Supported", "Supported", "TestDedicatedEnvelope_schema_renders_complete_envelope"},
	{"Exclusive generic/dedicated CRUD routing", "Supported", "Supported", "TestDedicatedStorageRouting_generated_helpers_have_authoritative_callers"},
	{"Explicit non-destructive migration helpers", "Supported", "Supported", "TestGeneratedMigration_is_explicit_and_dedicated_only"},
	{"Unique create/update conflict response", "HTTP 409", "HTTP 409", "TestGeneratedHandlers_map_create_and_update_storage_conflicts_to_stable_409"},
	{"Backend-common typed conflict contract", "Supported", "Supported", "TestGeneratedStorageErrors_define_backend_independent_conflict_contract"},
	{"Commit-aware migration continuation cursor", "Supported", "Supported", "TestDedicatedMigration_generated_SQLite_runtime"},
	{"Generated project generation, vet, and build", "Supported", "Supported", "TestGeneratedProjectMatrix_passes_generation_vet_and_build"},
	{"Generated SQLite runtime", "N/A", "Supported", "TestGeneratedSQLite_acceptance"},
	{"Restricted-role PostgreSQL runtime", "Supported", "N/A", "TestGeneratedPostgres_acceptance"},
}

var requiredUnsupported = []unsupportedRow{
	{"Encryption", "Rejected before output; no encryption or key-management runtime", "TestUnsupportedCapabilities_return_typed_source_error"},
	{"Argon2", "Rejected before output", "TestUnsupportedCapabilities_return_typed_source_error"},
	{"SHA-256 storage hashing", "Rejected before output", "TestUnsupportedCapabilities_return_typed_source_error"},
	{"MySQL dedicated annotations", "Rejected as an unsupported dialect", "TestUnsupportedCapabilities_rejects_mysql_dialect"},
	{"GiST", "Rejected for every supported dialect", "TestSecurityDialect_unsupported_crypto_types_and_indexes_return_capability_errors"},
	{"Unsupported Go types", "Rejected with a typed capability error", "TestUnsupportedCapabilities_return_typed_source_error"},
	{"Unknown Fabrica directives", "Rejected with a source-located typed parse error", "TestParseFileAnnotations_rejects_invalid_directives_with_source_context"},
}

var requiredBehaviorEvidence = []string{
	"TestGeneratedHandlers_bound_every_request_body_with_stable_413_contract",
	"TestDedicatedSecurity_generated_adapter_runtime",
	"TestGeneratedAnnotationProject_generic_storage_CRUD_and_queries_remain_compatible",
	"TestGeneratedLegacyHandlers_compile_for_file_and_ent_storage",
}

func TestDocumentation_contract_matches_executable_annotation_capabilities(t *testing.T) {
	// Given
	contract := loadDocumentationContract(t)

	// When
	errs := contract.validate()

	// Then
	for _, err := range errs {
		t.Error(err)
	}
}

func (c documentationContract) validate() []error {
	var errs []error
	pkg := c.documents["pkg/annotations/README.md"]
	errs = append(errs, requireTableRows(pkg, "Tested capability matrix", capabilityTableRows())...)
	errs = append(errs, requireTableRows(pkg, "Rejected capabilities", unsupportedTableRows())...)
	errs = append(errs, c.validateDocumentSections()...)
	errs = append(errs, c.validateContradictions()...)
	for _, row := range append(capabilityTableRows(), unsupportedTableRows()...) {
		evidence := row[len(row)-1]
		if _, ok := c.testSymbols[evidence]; !ok {
			errs = append(errs, fmt.Errorf("documented evidence test %q does not exist", evidence))
		}
	}
	for _, evidence := range requiredBehaviorEvidence {
		if _, ok := c.testSymbols[evidence]; !ok {
			errs = append(errs, fmt.Errorf("documented behavior evidence test %q does not exist", evidence))
		}
	}
	return errs
}

func (c documentationContract) validateContradictions() []error {
	forbidden := []string{
		"hashing (bcrypt, argon2, sha256)",
		"encryption (aes-128/192/256)",
		"all hashing/encryption algorithms are supported on all databases",
		"updates will return http 422",
		"annotation-driven dedicated storage supports mysql",
		"gist indexes are supported",
		"defaults are visible after reading persisted data, not in the create handler's immediate echo",
		"defaults are absent from the dedicated create response",
	}
	var errs []error
	for path, document := range c.documents {
		content := strings.ToLower(document.content)
		for _, claim := range forbidden {
			if strings.Contains(content, claim) {
				errs = append(errs, fmt.Errorf("%s contains contradictory annotation claim %q", path, claim))
			}
		}
	}
	return errs
}

func (c documentationContract) validateDocumentSections() []error {
	requirements := []sectionRequirement{
		{"README.md", "Annotation-driven dedicated Ent storage", []string{"field directives require dedicated Ent storage", "dedicated mode with file storage is rejected", "Resource version snapshots are file-backend only", "Every Ent resource with `+fabrica:resource-versioning=enabled` is rejected before output", "Unknown Fabrica directives fail with source-located typed parse errors", "historical annotation-layer proposal is non-authoritative", "docs/dev/ANNOTATION_LAYER_PROPOSAL.md", "required bcrypt values are enforced on create", "non-pointer sensitive zero values preserve storage", "reloads persisted resources before redacting write responses", "backend-common typed conflict contract", "cursor advances only after commit", "PostgreSQL and SQLite", "bcrypt is the only supported storage transform", "HTTP 409", "explicit and non-destructive", "Encryption, Argon2, SHA-256, MySQL, GiST, and unsupported Go types are rejected", "pkg/annotations/README.md"}},
		{"README.md", "Generated request-body and compatibility contract", []string{"16 MiB", "--request-body-max-bytes", "*_REQUEST_BODY_MAX_BYTES", "request_body_max_bytes", "request_body_max_bytes_by_resource", "exact resource Kind", "HTTP 413", "request body too large", "exactly one JSON value", "EOF", "trailing whitespace", "PATCH storage conflicts return HTTP 409", "Namespace and ResourceVersion", "legacy embedded-resource projects compile with file and Ent storage", "TestGeneratedHandlers_bound_every_request_body_with_stable_413_contract", "TestGeneratedLegacyHandlers_compile_for_file_and_ent_storage"}},
		{"CHANGELOG.md", "[Unreleased]", []string{"Field directives now fail closed unless the resource uses dedicated Ent storage", "Resource version snapshots are now explicitly file-backend only", "every Ent resource with versioning enabled fails before output", "Unknown Fabrica directives now fail strictly with source-located typed parse errors", "historical annotation-layer proposal is non-authoritative", "16 MiB", "--request-body-max-bytes", "service-prefixed environment variable", "request_body_max_bytes", "per-Kind", "HTTP 413", "EOF", "trailing", "PATCH storage conflicts", "HTTP 409", "Namespace and ResourceVersion", "legacy embedded-resource handlers", "file and Ent storage", "required bcrypt create versus omitted update", "persisted redacted write responses", "backend-common typed storage conflict", "commit-aware migration cursors"}},
		{"docs/README.md", "Guides (`guides/`)", []string{"**Core Features:**", "Storage Annotation Contract", "../pkg/annotations/README.md"}},
		{"docs/guides/middleware.md", "Request Body Limits", []string{"outermost router middleware", "16 MiB", "--request-body-max-bytes", "*_REQUEST_BODY_MAX_BYTES", "request_body_max_bytes", "request_body_max_bytes_by_resource", "exact generated resource Kind names", "HTTP 413", "request body too large", "exactly one JSON value", "verify EOF", "second value", "malformed trailing bytes", "trailing whitespace", "TestGeneratedHandlers_bound_every_request_body_with_stable_413_contract", "TestDedicatedSecurity_generated_adapter_runtime"}},
		{"docs/guides/storage.md", "Versioning support", []string{"Resource version snapshots are file-backend only", "Every Ent configuration with `+fabrica:resource-versioning=enabled` fails before output", "TestGeneratedFileVersioning_builds_and_runs_snapshot_runtime", "TestPrepareResourceAnnotations_rejects_ent_version_snapshots_for_every_storage_mode"}},
		{"docs/guides/storage-ent.md", "Annotation-driven dedicated schemas", []string{"Field directives are valid only for dedicated resources on the Ent backend", "dedicated mode with the file backend is rejected", "Resource version snapshots are file-backend only", "every generic or dedicated Ent resource with `+fabrica:resource-versioning=enabled` fails before output", "Unknown Fabrica directives fail with source-located typed parse errors", "Required bcrypt values on create", "omitted or redacted zero values on update preserve", "reloads the persisted entity", "backend-common typed storage conflict", "PATCH storage conflicts return HTTP 409", "generic Ent updates persist Namespace and ResourceVersion", "explicit zero values", "TestDedicatedSecurity_generated_adapter_runtime", "TestGeneratedAnnotationProject_generic_storage_CRUD_and_queries_remain_compatible", "cursor advances only after its transaction commits", "supports PostgreSQL and SQLite only", "explicit and non-destructive", "never delete generic source rows", "Encryption, Argon2, SHA-256, MySQL, GiST, and unsupported Go types are rejected"}},
		{"examples/README.md", "10. [Storage Annotations](12-storage-annotations/) - Dedicated Ent Schemas 🗃️", []string{"generate a dedicated `User` entity", "PostgreSQL and SQLite", "Encryption, Argon2, SHA-256, MySQL, GiST, and unsupported field types are rejected"}},
		{"examples/12-storage-annotations/README.md", "Proven capabilities", []string{"Field directives require dedicated Ent storage", "dedicated mode with the file backend is rejected", "Resource version snapshots are file-backend only", "this Ent example rejects `+fabrica:resource-versioning=enabled` before output", "Unknown Fabrica directives fail strictly", "Required bcrypt input is mandatory on create", "non-pointer zero values are treated as omitted and preserve storage", "non-nil pointers explicitly replace storage, including with zero", "reload persisted dedicated records before returning redacted responses", "backend-common typed conflict contract", "dedicated Ent storage", "bcrypt is the only supported storage transform", "Encryption, Argon2, SHA-256, MySQL, GiST, and unsupported Go types are rejected"}},
		{"examples/12-storage-annotations/README.md", "Executable API contract", []string{"defaults are present in the dedicated create response after persisted reload", "create_persisted_defaults"}},
		{"examples/12-storage-annotations/README.md", "Migration and cutover", []string{"cursor advances only after the batch transaction commits", "rollback returns the input cursor and zero copied rows", "explicit and non-destructive", "does not delete the generic source rows"}},
	}
	var errs []error
	for _, requirement := range requirements {
		document := c.documents[requirement.path]
		section, ok := document.sections[requirement.heading]
		if !ok {
			errs = append(errs, fmt.Errorf("%s missing section %q", requirement.path, requirement.heading))
			continue
		}
		for _, claim := range requirement.claims {
			if !strings.Contains(section, claim) {
				errs = append(errs, fmt.Errorf("%s section %q missing claim %q", requirement.path, requirement.heading, claim))
			}
		}
	}
	docGo := c.documents["pkg/annotations/doc.go"].content
	for _, linkage := range []string{"README.md", "Tested capability matrix", "dedicated Ent storage", "file-backend only", "Ent", "versioning", "unknown", "source-located typed", "required bcrypt", "redacted write responses", "backend-common conflict", "cursor", "commit", "rollback", "TestGeneratedSQLite_acceptance", "TestGeneratedPostgres_acceptance"} {
		if !strings.Contains(docGo, linkage) {
			errs = append(errs, fmt.Errorf("pkg/annotations/doc.go missing package contract linkage %q", linkage))
		}
	}
	return errs
}

func capabilityTableRows() [][]string {
	rows := make([][]string, 0, len(requiredCapabilities))
	for _, row := range requiredCapabilities {
		rows = append(rows, []string{row.capability, row.postgres, row.sqlite, row.evidence})
	}
	return rows
}

func unsupportedTableRows() [][]string {
	rows := make([][]string, 0, len(requiredUnsupported))
	for _, row := range requiredUnsupported {
		rows = append(rows, []string{row.capability, row.contract, row.evidence})
	}
	return rows
}
