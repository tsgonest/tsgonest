package compiler

import (
	"github.com/microsoft/TypeScript/tsc/shim/bundled"
	shimcompiler "github.com/microsoft/TypeScript/tsc/shim/compiler"
	"github.com/microsoft/TypeScript/tsc/shim/vfs"
	"github.com/microsoft/TypeScript/tsc/shim/vfs/cachedvfs"
	"github.com/microsoft/TypeScript/tsc/shim/vfs/osvfs"
)

// CreateDefaultFS creates a filesystem using the OS filesystem with bundled libs.
func CreateDefaultFS() vfs.FS {
	return bundled.WrapFS(cachedvfs.From(osvfs.FS()))
}

// CreateDefaultHost creates a compiler host with default settings.
func CreateDefaultHost(fs vfs.FS) shimcompiler.CompilerHost {
	return shimcompiler.NewCompilerHost(fs, bundled.LibPath(), nil, nil, nil)
}
