// Package schemas embeds the language-neutral tool contract shared by the
// Go CLI (this embed) and the Node server (build-time codegen).
package schemas

import _ "embed"

//go:embed tools.json
var ToolsJSON []byte
