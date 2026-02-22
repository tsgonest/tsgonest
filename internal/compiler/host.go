package compiler

import (
	"github.com/microsoft/typescript-go/shim/bundled"
	shimcompiler "github.com/microsoft/typescript-go/shim/compiler"
	"github.com/microsoft/typescript-go/shim/vfs"
	"github.com/microsoft/typescript-go/shim/vfs/cachedvfs"
	"github.com/microsoft/typescript-go/shim/vfs/osvfs"
)

// CreateDefaultFS creates a filesystem using the OS filesystem with bundled libs.
func CreateDefaultFS() vfs.FS {
	return bundled.WrapFS(cachedvfs.From(osvfs.FS()))
}

// CreateDefaultHost creates a compiler host with default settings.
func CreateDefaultHost(cwd string, fs vfs.FS) shimcompiler.CompilerHost {
	return shimcompiler.NewCompilerHost(cwd, fs, bundled.LibPath(), nil, nil)
}
