package workstation

import (
	"embed"
)

// EmbeddedConfigFS provides filesystem access to embedded config files.
//
//go:embed config/toolchains.json
var EmbeddedConfigFS embed.FS

// EmbeddedToolchainsJSON contains the raw bytes of embedded config/toolchains.json.
//
//go:embed config/toolchains.json
var EmbeddedToolchainsJSON []byte
