module github.com/tsgonest/tsgonest

go 1.27

replace (
	github.com/microsoft/TypeScript/tsc/shim/ast => ./shim/ast
	github.com/microsoft/TypeScript/tsc/shim/bundled => ./shim/bundled
	github.com/microsoft/TypeScript/tsc/shim/checker => ./shim/checker
	github.com/microsoft/TypeScript/tsc/shim/compiler => ./shim/compiler
	github.com/microsoft/TypeScript/tsc/shim/core => ./shim/core
	github.com/microsoft/TypeScript/tsc/shim/execute/incremental => ./shim/execute/incremental
	github.com/microsoft/TypeScript/tsc/shim/fswatch => ./shim/fswatch
	github.com/microsoft/TypeScript/tsc/shim/parser => ./shim/parser
	github.com/microsoft/TypeScript/tsc/shim/scanner => ./shim/scanner
	github.com/microsoft/TypeScript/tsc/shim/tsoptions => ./shim/tsoptions
	github.com/microsoft/TypeScript/tsc/shim/tspath => ./shim/tspath
	github.com/microsoft/TypeScript/tsc/shim/vfs => ./shim/vfs
	github.com/microsoft/TypeScript/tsc/shim/vfs/cachedvfs => ./shim/vfs/cachedvfs
	github.com/microsoft/TypeScript/tsc/shim/vfs/osvfs => ./shim/vfs/osvfs
)

require (
	github.com/Microsoft/go-winio v0.6.4 // indirect
	github.com/klauspost/compress v1.20.3 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/microsoft/TypeScript/tsc v0.0.0-20261111233104-ad24728023c5 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
)

require (
	github.com/bmatcuk/doublestar/v4 v4.10.0
	github.com/microsoft/TypeScript/tsc/shim/ast v0.0.0-00010101000000-000000000000
	github.com/microsoft/TypeScript/tsc/shim/bundled v0.0.0-00010101000000-000000000000
	github.com/microsoft/TypeScript/tsc/shim/checker v0.0.0-00010101000000-000000000000
	github.com/microsoft/TypeScript/tsc/shim/compiler v0.0.0-00010101000000-000000000000
	github.com/microsoft/TypeScript/tsc/shim/core v0.0.0-00010101000000-000000000000
	github.com/microsoft/TypeScript/tsc/shim/execute/incremental v0.0.0-00010101000000-000000000000
	github.com/microsoft/TypeScript/tsc/shim/fswatch v0.0.0-00010101000000-000000000000
	github.com/microsoft/TypeScript/tsc/shim/parser v0.0.0-00010101000000-000000000000
	github.com/microsoft/TypeScript/tsc/shim/scanner v0.0.0-00010101000000-000000000000
	github.com/microsoft/TypeScript/tsc/shim/tsoptions v0.0.0-00010101000000-000000000000
	github.com/microsoft/TypeScript/tsc/shim/tspath v0.0.0-00010101000000-000000000000
	github.com/microsoft/TypeScript/tsc/shim/vfs v0.0.0-00010101000000-000000000000
	github.com/microsoft/TypeScript/tsc/shim/vfs/cachedvfs v0.0.0-00010101000000-000000000000
	github.com/microsoft/TypeScript/tsc/shim/vfs/osvfs v0.0.0-00010101000000-000000000000
	golang.org/x/sys v0.49.0
	golang.org/x/text v0.43.0
	golang.org/x/tools v0.52.0
)
