package web

import "embed"

// FS holds the embedded single-page UI.
//
//go:embed index.html css js
var FS embed.FS
