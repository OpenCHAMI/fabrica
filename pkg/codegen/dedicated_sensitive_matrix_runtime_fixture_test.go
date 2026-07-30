// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package codegen

const dedicatedSensitiveMatrixRuntimeTest = `

func requireSensitiveMatrixRedacted(t *testing.T, got *v1.Token) {
	t.Helper()
	spec := got.Spec
	if spec.Password != "" || spec.OptionalKey != "" || spec.ImmutableSecret != "" || spec.SensitiveNote != "" || spec.SensitiveBool || spec.SensitiveInt != 0 || spec.SensitiveInt64 != 0 || spec.SensitiveFloat64 != 0 || !spec.SensitiveTime.IsZero() || spec.SensitiveStrings != nil || spec.SensitiveStringPtr != nil || spec.SensitiveBoolPtr != nil || spec.SensitiveIntPtr != nil || spec.SensitiveInt64Ptr != nil || spec.SensitiveFloat64Ptr != nil || spec.SensitiveTimePtr != nil {
		t.Fatalf("sensitive matrix was not redacted: %#v", spec)
	}
}

func TestDedicatedSecurity_sensitive_type_matrix_presence_semantics(t *testing.T) {
	// Given
	client, _ := openSecurityDB(t)
	SetEntClient(client)
	input := securityToken("sensitive-matrix")
	if err := SaveToken(t.Context(), input); err != nil { t.Fatal(err) }
	created, err := client.Token.Query().Where(token.UIDEQ(input.Metadata.UID)).Only(t.Context())
	if err != nil { t.Fatal(err) }
	redacted, err := LoadToken(t.Context(), input.Metadata.UID)
	if err != nil { t.Fatal(err) }
	requireSensitiveMatrixRedacted(t, redacted)
	passwordBefore := created.SpecPassword
	stringBefore, boolBefore := created.SpecSensitiveNote, created.SpecSensitiveBool
	intBefore, int64Before := created.SpecSensitiveInt, created.SpecSensitiveInt64
	floatBefore, timeBefore := created.SpecSensitiveFloat64, created.SpecSensitiveTime
	stringsBefore := append([]string(nil), created.SpecSensitiveStrings...)
	redacted.Spec.SensitiveStrings = []string{}

	// When: redacted non-pointer zeros and an empty slice are saved.
	if err := SaveToken(t.Context(), redacted); err != nil { t.Fatal(err) }
	preserved, err := client.Token.Query().Where(token.UIDEQ(input.Metadata.UID)).Only(t.Context())
	if err != nil { t.Fatal(err) }

	// Then: ambiguous zero values preserve every existing non-pointer value.
	if preserved.SpecPassword != passwordBefore || preserved.SpecSensitiveNote != stringBefore || preserved.SpecSensitiveBool != boolBefore || preserved.SpecSensitiveInt != intBefore || preserved.SpecSensitiveInt64 != int64Before || preserved.SpecSensitiveFloat64 != floatBefore || !preserved.SpecSensitiveTime.Equal(timeBefore) || strings.Join(preserved.SpecSensitiveStrings, "|") != strings.Join(stringsBefore, "|") { t.Fatal("ambiguous zero-value update changed sensitive columns") }
	if preserved.SpecSensitiveStringPtr == nil || created.SpecSensitiveStringPtr == nil || *preserved.SpecSensitiveStringPtr != *created.SpecSensitiveStringPtr || preserved.SpecSensitiveBoolPtr == nil || created.SpecSensitiveBoolPtr == nil || *preserved.SpecSensitiveBoolPtr != *created.SpecSensitiveBoolPtr || preserved.SpecSensitiveIntPtr == nil || created.SpecSensitiveIntPtr == nil || *preserved.SpecSensitiveIntPtr != *created.SpecSensitiveIntPtr || preserved.SpecSensitiveInt64Ptr == nil || created.SpecSensitiveInt64Ptr == nil || *preserved.SpecSensitiveInt64Ptr != *created.SpecSensitiveInt64Ptr || preserved.SpecSensitiveFloat64Ptr == nil || created.SpecSensitiveFloat64Ptr == nil || *preserved.SpecSensitiveFloat64Ptr != *created.SpecSensitiveFloat64Ptr || preserved.SpecSensitiveTimePtr == nil || created.SpecSensitiveTimePtr == nil || !preserved.SpecSensitiveTimePtr.Equal(*created.SpecSensitiveTimePtr) { t.Fatal("nil pointer omission changed sensitive columns") }
	readAfterPreserve, err := LoadToken(t.Context(), input.Metadata.UID)
	if err != nil { t.Fatal(err) }
	requireSensitiveMatrixRedacted(t, readAfterPreserve)

	// Given: explicit nonzero replacements for every non-pointer form.
	readAfterPreserve.Spec.SensitiveNote = "replaced-string"
	readAfterPreserve.Spec.SensitiveBool = true
	readAfterPreserve.Spec.SensitiveInt = 31
	readAfterPreserve.Spec.SensitiveInt64 = 32
	readAfterPreserve.Spec.SensitiveFloat64 = 33.5
	readAfterPreserve.Spec.SensitiveTime = time.Date(2026, 7, 21, 1, 2, 3, 0, time.UTC)
	readAfterPreserve.Spec.SensitiveStrings = []string{"replaced"}

	// When
	if err := SaveToken(t.Context(), readAfterPreserve); err != nil { t.Fatal(err) }
	replaced, err := client.Token.Query().Where(token.UIDEQ(input.Metadata.UID)).Only(t.Context())
	if err != nil { t.Fatal(err) }

	// Then
	if replaced.SpecSensitiveNote != "replaced-string" || !replaced.SpecSensitiveBool || replaced.SpecSensitiveInt != 31 || replaced.SpecSensitiveInt64 != 32 || replaced.SpecSensitiveFloat64 != 33.5 || !replaced.SpecSensitiveTime.Equal(readAfterPreserve.Spec.SensitiveTime) || strings.Join(replaced.SpecSensitiveStrings, "|") != "replaced" { t.Fatal("nonzero sensitive replacements were not persisted") }
	readAfterReplace, err := LoadToken(t.Context(), input.Metadata.UID)
	if err != nil { t.Fatal(err) }
	requireSensitiveMatrixRedacted(t, readAfterReplace)

	zeroString, zeroBool, zeroInt := "", false, 0
	zeroInt64, zeroFloat, zeroTime := int64(0), float64(0), time.Time{}
	readAfterReplace.Spec.SensitiveStringPtr = &zeroString
	readAfterReplace.Spec.SensitiveBoolPtr = &zeroBool
	readAfterReplace.Spec.SensitiveIntPtr = &zeroInt
	readAfterReplace.Spec.SensitiveInt64Ptr = &zeroInt64
	readAfterReplace.Spec.SensitiveFloat64Ptr = &zeroFloat
	readAfterReplace.Spec.SensitiveTimePtr = &zeroTime

	// When: supported pointers explicitly carry zero values.
	if err := SaveToken(t.Context(), readAfterReplace); err != nil { t.Fatal(err) }
	pointerZeros, err := client.Token.Query().Where(token.UIDEQ(input.Metadata.UID)).Only(t.Context())
	if err != nil { t.Fatal(err) }

	// Then
	if pointerZeros.SpecSensitiveStringPtr == nil || *pointerZeros.SpecSensitiveStringPtr != "" || pointerZeros.SpecSensitiveBoolPtr == nil || *pointerZeros.SpecSensitiveBoolPtr || pointerZeros.SpecSensitiveIntPtr == nil || *pointerZeros.SpecSensitiveIntPtr != 0 || pointerZeros.SpecSensitiveInt64Ptr == nil || *pointerZeros.SpecSensitiveInt64Ptr != 0 || pointerZeros.SpecSensitiveFloat64Ptr == nil || *pointerZeros.SpecSensitiveFloat64Ptr != 0 || pointerZeros.SpecSensitiveTimePtr == nil || !pointerZeros.SpecSensitiveTimePtr.IsZero() { t.Fatal("explicit pointer zero values were not persisted") }
	readAfterPointerZeros, err := LoadToken(t.Context(), input.Metadata.UID)
	if err != nil { t.Fatal(err) }
	requireSensitiveMatrixRedacted(t, readAfterPointerZeros)
	t.Log("sensitive type matrix")
}
`
