package codegen

import "testing"

// TestRelativeCompanionImport_ForwardSlash verifies the happy path: both
// inputs are forward-slash relative paths (the canonical form on Unix).
func TestRelativeCompanionImport_ForwardSlash(t *testing.T) {
	cases := []struct {
		from string
		to   string
		want string
	}{
		{
			// Same directory.
			from: "dist/channel/dto/channel.dto.A.tsgonest.js",
			to:   "dist/channel/dto/channel.dto.B.tsgonest.js",
			want: "./channel.dto.B.tsgonest.js",
		},
		{
			// Sibling directory.
			from: "dist/channel/channel.controller.A.tsgonest.js",
			to:   "dist/channel/dto/channel.dto.B.tsgonest.js",
			want: "./dto/channel.dto.B.tsgonest.js",
		},
		{
			// Parent directory.
			from: "dist/channel/dto/channel.dto.A.tsgonest.js",
			to:   "dist/shared.B.tsgonest.js",
			want: "../../shared.B.tsgonest.js",
		},
	}

	for _, c := range cases {
		got := relativeCompanionImport(c.from, c.to)
		if got != c.want {
			t.Errorf("relativeCompanionImport(%q, %q) = %q; want %q", c.from, c.to, got, c.want)
		}
	}
}

// TestRelativeCompanionImport_WindowsBackslash_Issue140 exercises the Windows
// bug from issue #140. The codegen layer previously used Go's `path` package
// (forward-slash semantics) on inputs that came from `filepath.Join` on
// Windows (backslash output). The result was an `import` path that started
// with `./` glued onto an absolute Windows path:
//
//	"./D:\Web_Development\...\channel.dto.B.tsgonest.js"
//
// causing `Cannot find module` at runtime.
//
// Regression test: feed in backslashed paths exactly as they appear on
// Windows and assert the import path is a valid forward-slash relative path
// with no absolute-path leak.
func TestRelativeCompanionImport_WindowsBackslash_Issue140(t *testing.T) {
	cases := []struct {
		name string
		from string
		to   string
		want string
	}{
		{
			name: "absolute Windows path, same dir",
			from: `D:\Web_Development\proj\dist\channel\dto\channel.dto.A.tsgonest.js`,
			to:   `D:\Web_Development\proj\dist\channel\dto\channel.dto.B.tsgonest.js`,
			want: "./channel.dto.B.tsgonest.js",
		},
		{
			name: "absolute Windows path, sibling dir",
			from: `D:\Web_Development\proj\dist\channel\channel.controller.A.tsgonest.js`,
			to:   `D:\Web_Development\proj\dist\channel\dto\channel.dto.B.tsgonest.js`,
			want: "./dto/channel.dto.B.tsgonest.js",
		},
		{
			name: "absolute Windows path, parent dir",
			from: `D:\Web_Development\proj\dist\channel\dto\channel.dto.A.tsgonest.js`,
			to:   `D:\Web_Development\proj\dist\shared.B.tsgonest.js`,
			want: "../../shared.B.tsgonest.js",
		},
		{
			name: "mixed separators (Git Bash on Windows)",
			from: `C:/proj/dist\channel\channel.dto.A.tsgonest.js`,
			to:   `C:/proj\dist/channel\dto/channel.dto.B.tsgonest.js`,
			want: "./dto/channel.dto.B.tsgonest.js",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := relativeCompanionImport(c.from, c.to)
			if got != c.want {
				t.Errorf("relativeCompanionImport(%q, %q)\n  got:  %q\n  want: %q", c.from, c.to, got, c.want)
			}
			// Defensive: never produce an output with backslashes or an
			// absolute-path leak (drive letter, leading `/`, etc.).
			if containsBackslash(got) {
				t.Errorf("output contains backslash: %q", got)
			}
		})
	}
}

func containsBackslash(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			return true
		}
	}
	return false
}
