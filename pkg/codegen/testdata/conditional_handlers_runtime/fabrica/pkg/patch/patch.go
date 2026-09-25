// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package patch

type PatchOptions struct {
	AllowAddFields    bool
	AllowRemoveFields bool
}

type Result struct {
	Updated []byte
}

func DetectPatchType(contentType string) string {
	return contentType
}

func ApplyPatchWithOptions(_ []byte, patchData []byte, _ string, _ PatchOptions) (Result, error) {
	return Result{Updated: patchData}, nil
}
