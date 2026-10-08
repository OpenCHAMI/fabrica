// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

module example.com/test

go 1.26.6

require (
	github.com/go-chi/chi/v5 v5.0.0
	github.com/openchami/fabrica v0.0.0
)

replace github.com/go-chi/chi/v5 => ./chi

replace github.com/openchami/fabrica => ./fabrica
