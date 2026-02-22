// Package testutil provides test utilities for tsgonest, including a virtual
// filesystem overlay for creating tsgo programs from inline TypeScript source.
package testutil

import (
	"io/fs"
	"strings"
	"time"

	"github.com/microsoft/typescript-go/shim/bundled"
	"github.com/microsoft/typescript-go/shim/tspath"
	"github.com/microsoft/typescript-go/shim/vfs"
	"github.com/microsoft/typescript-go/shim/vfs/osvfs"
)

// OverlayVFS wraps a base filesystem with in-memory virtual files.
// Virtual files take precedence over the underlying filesystem.
type OverlayVFS struct {
	fs           vfs.FS
	VirtualFiles map[string]string
}

var _ vfs.FS = (*OverlayVFS)(nil)

func (o *OverlayVFS) UseCaseSensitiveFileNames() bool {
	return o.fs.UseCaseSensitiveFileNames()
}

func (o *OverlayVFS) FileExists(path string) bool {
	if _, ok := o.VirtualFiles[path]; ok {
		return true
	}
	return o.fs.FileExists(path)
}

func (o *OverlayVFS) ReadFile(path string) (contents string, ok bool) {
	if src, ok := o.VirtualFiles[path]; ok {
		return src, true
	}
	return o.fs.ReadFile(path)
}

func (o *OverlayVFS) DirectoryExists(path string) bool {
	normalizedPath := tspath.NormalizePath(path)
	if !strings.HasSuffix(normalizedPath, "/") {
		normalizedPath = normalizedPath + "/"
	}
	for virtualFilePath := range o.VirtualFiles {
		if strings.HasPrefix(virtualFilePath, normalizedPath) {
			return true
		}
	}
	return o.fs.DirectoryExists(path)
}

func (o *OverlayVFS) GetAccessibleEntries(path string) (result vfs.Entries) {
	result = o.fs.GetAccessibleEntries(path)

	normalizedPath := tspath.NormalizePath(path)
	if !strings.HasSuffix(normalizedPath, "/") {
		normalizedPath = normalizedPath + "/"
	}

	for virtualFilePath := range o.VirtualFiles {
		withoutPrefix, found := strings.CutPrefix(virtualFilePath, normalizedPath)
		if !found {
			continue
		}
		if before, _, ok := strings.Cut(withoutPrefix, "/"); ok {
			result.Directories = append(result.Directories, before)
		} else {
			result.Files = append(result.Files, withoutPrefix)
		}
	}
	return result
}

type overlayFileInfo struct {
	mode fs.FileMode
	name string
	size int64
}

var (
	_ fs.FileInfo = (*overlayFileInfo)(nil)
	_ fs.DirEntry = (*overlayFileInfo)(nil)
)

func (fi *overlayFileInfo) IsDir() bool                { return fi.mode.IsDir() }
func (fi *overlayFileInfo) ModTime() time.Time         { return time.Time{} }
func (fi *overlayFileInfo) Mode() fs.FileMode          { return fi.mode }
func (fi *overlayFileInfo) Name() string               { return fi.name }
func (fi *overlayFileInfo) Size() int64                { return fi.size }
func (fi *overlayFileInfo) Sys() any                   { return nil }
func (fi *overlayFileInfo) Info() (fs.FileInfo, error) { return fi, nil }
func (fi *overlayFileInfo) Type() fs.FileMode          { return fi.mode.Type() }

func (o *OverlayVFS) Stat(path string) vfs.FileInfo {
	if src, ok := o.VirtualFiles[path]; ok {
		return &overlayFileInfo{
			name: path,
			size: int64(len(src)),
		}
	}
	return o.fs.Stat(path)
}

func (o *OverlayVFS) WalkDir(root string, walkFn vfs.WalkDirFunc) error {
	return o.fs.WalkDir(root, walkFn)
}

func (o *OverlayVFS) Realpath(path string) string {
	if _, ok := o.VirtualFiles[path]; ok {
		return path
	}
	return o.fs.Realpath(path)
}

func (o *OverlayVFS) WriteFile(path string, data string, writeByteOrderMark bool) error {
	if _, ok := o.VirtualFiles[path]; ok {
		panic("cannot write to overlay virtual file")
	}
	return o.fs.WriteFile(path, data, writeByteOrderMark)
}

func (o *OverlayVFS) Remove(path string) error {
	if _, ok := o.VirtualFiles[path]; ok {
		panic("cannot remove overlay virtual file")
	}
	return o.fs.Remove(path)
}

func (o *OverlayVFS) Chtimes(path string, aTime time.Time, mTime time.Time) error {
	if _, ok := o.VirtualFiles[path]; ok {
		panic("cannot change times on overlay virtual file")
	}
	return o.fs.Chtimes(path, aTime, mTime)
}

// NewOverlayVFS creates an OverlayVFS with the given virtual files on top of a base FS.
func NewOverlayVFS(baseFS vfs.FS, virtualFiles map[string]string) vfs.FS {
	return &OverlayVFS{baseFS, virtualFiles}
}

// NewDefaultOverlayVFS creates an OverlayVFS with virtual files on top of the
// bundled OS filesystem (includes TypeScript lib files).
func NewDefaultOverlayVFS(virtualFiles map[string]string) vfs.FS {
	return &OverlayVFS{bundled.WrapFS(osvfs.FS()), virtualFiles}
}
