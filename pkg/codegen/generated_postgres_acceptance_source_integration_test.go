// Copyright © 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

//go:build integration

package codegen

import "strings"

func generatedPostgresTokenSource() string {
	const insertionPoint = "\tOptionalNote *string `json:\"optionalNote,omitempty\"`\n"
	const postgresFields = insertionPoint + `
	// +fabrica:field:index=gin
	Tags []string ` + "`json:\"tags,omitempty\"`" + `
	// +fabrica:field:index=hash
	Bucket string ` + "`json:\"bucket\"`" + `
	// +fabrica:field:storage=hashed:bcrypt:cost=4
	// +fabrica:field:sensitive
	Password string ` + "`json:\"password\"`" + `
`
	return strings.Replace(generatedSQLiteTokenSource, insertionPoint, postgresFields, 1)
}
