<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# TypeID conformance vectors

`valid.yml` and `invalid.yml` are copied without modification from
`go.jetify.com/typeid/v2@v2.0.0-alpha.3/testdata`, whose README identifies them
as copies of the [official TypeID specification vectors](https://github.com/jetify-com/typeid).
The files target specification 0.3.0; the invalid file also contains the Go
implementation's additional `suffix-dot` case. The upstream Apache-2.0
license is included as `LICENSES/Apache-2.0.txt` at the repository root.

Retain these vectors and provenance when upgrading the dependency. Valid
parsing vectors intentionally include non-v7 and nil UUIDs. Newly generated
UIDs must contain RFC 9562 UUIDv7 values even though parsing accepts those
other payloads.
