package compiler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/microsoft/typescript-go/shim/ast"
	shimast "github.com/microsoft/typescript-go/shim/ast"
	shimscanner "github.com/microsoft/typescript-go/shim/scanner"
)

// DiagnosticCategory mirrors tsgo's diagnostics.Category.
// We redeclare here to avoid importing the internal diagnostics package directly.
type DiagnosticCategory int

const (
	CategoryWarning    DiagnosticCategory = 0
	CategoryError      DiagnosticCategory = 1
	CategorySuggestion DiagnosticCategory = 2
	CategoryMessage    DiagnosticCategory = 3
)

func (c DiagnosticCategory) Name() string {
	switch c {
	case CategoryError:
		return "error"
	case CategoryWarning:
		return "warning"
	case CategorySuggestion:
		return "suggestion"
	case CategoryMessage:
		return "message"
	}
	return "unknown"
}

// ANSI color constants matching tsgo's diagnosticwriter.
const (
	colorReset  = "\u001b[0m"
	colorRed    = "\u001b[91m"
	colorYellow = "\u001b[93m"
	colorCyan   = "\u001b[96m"
	colorGrey   = "\u001b[90m"
	colorGutter = "\u001b[7m" // reverse video
)

// categoryColor returns the ANSI escape for a diagnostic category.
func categoryColor(cat DiagnosticCategory) string {
	switch cat {
	case CategoryError:
		return colorRed
	case CategoryWarning:
		return colorYellow
	case CategorySuggestion:
		return colorGrey
	case CategoryMessage:
		return "\u001b[94m" // blue
	}
	return ""
}

// DiagnosticReporter formats and writes a single diagnostic to a writer.
type DiagnosticReporter func(d *ast.Diagnostic)

// IsPrettyOutput determines if we should use colored output with code snippets.
// Mirrors tsgo's shouldBePretty logic: NO_COLOR, FORCE_COLOR, then isatty.
func IsPrettyOutput() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	// Check if stderr is a terminal
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// CreateDiagnosticReporter creates a reporter that formats diagnostics in tsc style.
// When pretty=true, outputs colored diagnostics with code snippets (like tsgo).
// When pretty=false, outputs plain diagnostics: file(line,col): category TScode: message
func CreateDiagnosticReporter(w io.Writer, cwd string, pretty bool) DiagnosticReporter {
	if pretty {
		return func(d *ast.Diagnostic) {
			writePrettyDiagnostic(w, d, cwd)
			fmt.Fprint(w, "\n")
		}
	}
	return func(d *ast.Diagnostic) {
		writePlainDiagnostic(w, d, cwd)
	}
}

// writePlainDiagnostic writes a diagnostic in tsc plain format:
// file(line,col): error TS2322: message
func writePlainDiagnostic(w io.Writer, d *ast.Diagnostic, cwd string) {
	if d.File() != nil {
		line, char := shimscanner.GetECMALineAndCharacterOfPosition(d.File(), d.Pos())
		fileName := relativePath(d.File().FileName(), cwd)
		fmt.Fprintf(w, "%s(%d,%d): ", fileName, line+1, char+1)
	}

	cat := DiagnosticCategory(shimast.Diagnostic_Category(d))
	fmt.Fprintf(w, "%s TS%d: %s\n", cat.Name(), d.Code(), d.String())
}

// writePrettyDiagnostic writes a diagnostic in tsgo's pretty format:
// file:line:col - error TS2322: message
// <code snippet with squiggles>
func writePrettyDiagnostic(w io.Writer, d *ast.Diagnostic, cwd string) {
	cat := DiagnosticCategory(shimast.Diagnostic_Category(d))

	if d.File() != nil {
		line, char := shimscanner.GetECMALineAndCharacterOfPosition(d.File(), d.Pos())
		fileName := relativePath(d.File().FileName(), cwd)
		// file:line:col colored like tsgo
		fmt.Fprintf(w, "%s%s%s:%s%d%s:%s%d%s",
			colorCyan, fileName, colorReset,
			colorYellow, line+1, colorReset,
			colorYellow, char+1, colorReset)
		fmt.Fprint(w, " - ")
	}

	// category colored + TScode + message
	fmt.Fprintf(w, "%s%s%s %sTS%d:%s %s",
		categoryColor(cat), cat.Name(), colorReset,
		colorGrey, d.Code(), colorReset,
		d.String())

	// Code snippet with squiggles
	if d.File() != nil && d.Len() > 0 {
		fmt.Fprint(w, "\n")
		writeCodeSnippet(w, d.File(), d.Pos(), d.Len(), categoryColor(cat))
		fmt.Fprint(w, "\n")
	}
}

// writeCodeSnippet writes the source code context with gutter line numbers and squiggles.
// Mirrors tsgo's diagnosticwriter.writeCodeSnippet.
func writeCodeSnippet(w io.Writer, file *ast.SourceFile, start int, length int, squiggleColor string) {
	firstLine, firstLineChar := shimscanner.GetECMALineAndCharacterOfPosition(file, start)
	lastLine, lastLineChar := shimscanner.GetECMALineAndCharacterOfPosition(file, start+length)
	if length == 0 {
		lastLineChar++
	}

	text := file.Text()
	lastLineOfFile := shimscanner.GetECMALineOfPosition(file, len(text))

	hasMoreThanFiveLines := lastLine-firstLine >= 4
	gutterWidth := len(strconv.Itoa(lastLine + 1))
	if hasMoreThanFiveLines && len("...") > gutterWidth {
		gutterWidth = len("...")
	}

	for i := firstLine; i <= lastLine; i++ {
		if hasMoreThanFiveLines && firstLine+1 < i && i < lastLine-1 {
			fmt.Fprintf(w, "%s%*s%s %s\n",
				colorGutter, gutterWidth, "...", colorReset, "")
			i = lastLine - 1
		}

		lineStart := shimscanner.GetECMAPositionOfLineAndCharacter(file, i, 0)
		var lineEnd int
		if i < lastLineOfFile {
			lineEnd = shimscanner.GetECMAPositionOfLineAndCharacter(file, i+1, 0)
		} else {
			lineEnd = len(text)
		}

		lineContent := strings.TrimRightFunc(text[lineStart:lineEnd], unicode.IsSpace)
		lineContent = strings.ReplaceAll(lineContent, "\t", " ")

		// Gutter + line content
		fmt.Fprintf(w, "%s%*d%s %s\n",
			colorGutter, gutterWidth, i+1, colorReset, lineContent)

		// Squiggles
		fmt.Fprintf(w, "%s%*s%s ", colorGutter, gutterWidth, "", colorReset)
		fmt.Fprint(w, squiggleColor)
		switch i {
		case firstLine:
			lastCharForLine := lastLineChar
			if i != lastLine {
				lastCharForLine = len(lineContent)
			}
			fmt.Fprint(w, strings.Repeat(" ", firstLineChar))
			squiggleLen := lastCharForLine - firstLineChar
			if squiggleLen < 1 {
				squiggleLen = 1
			}
			fmt.Fprint(w, strings.Repeat("~", squiggleLen))
		case lastLine:
			if lastLineChar > 0 {
				fmt.Fprint(w, strings.Repeat("~", lastLineChar))
			}
		default:
			fmt.Fprint(w, strings.Repeat("~", len(lineContent)))
		}
		fmt.Fprint(w, colorReset)
	}
}

// WriteErrorSummary writes the "Found N errors" summary (pretty mode only).
// Matches tsgo's error summary — only counts CategoryError diagnostics.
func WriteErrorSummary(w io.Writer, diags []*ast.Diagnostic, cwd string) {
	errorCount := 0
	var firstErrorFile *ast.SourceFile
	var firstErrorPos int
	fileErrors := make(map[string]int) // fileName -> count

	for _, d := range diags {
		cat := DiagnosticCategory(shimast.Diagnostic_Category(d))
		if cat != CategoryError {
			continue
		}
		errorCount++
		if errorCount == 1 && d.File() != nil {
			firstErrorFile = d.File()
			firstErrorPos = d.Pos()
		}
		if d.File() != nil {
			fileErrors[d.File().FileName()]++
		}
	}

	if errorCount == 0 {
		return
	}

	numFiles := len(fileErrors)
	fmt.Fprint(w, "\n")

	if errorCount == 1 {
		if firstErrorFile != nil {
			line := shimscanner.GetECMALineOfPosition(firstErrorFile, firstErrorPos)
			fileName := relativePath(firstErrorFile.FileName(), cwd)
			fmt.Fprintf(w, "Found 1 error in %s%s:%d%s\n",
				fileName, colorGrey, line+1, colorReset)
		} else {
			fmt.Fprintln(w, "Found 1 error.")
		}
	} else if numFiles <= 1 {
		if firstErrorFile != nil {
			line := shimscanner.GetECMALineOfPosition(firstErrorFile, firstErrorPos)
			fileName := relativePath(firstErrorFile.FileName(), cwd)
			fmt.Fprintf(w, "Found %d errors in the same file, starting at: %s%s:%d%s\n",
				errorCount, fileName, colorGrey, line+1, colorReset)
		} else {
			fmt.Fprintf(w, "Found %d errors.\n", errorCount)
		}
	} else {
		fmt.Fprintf(w, "Found %d errors in %d files.\n", errorCount, numFiles)
	}
	fmt.Fprint(w, "\n")
}

// CountErrors returns the number of CategoryError diagnostics.
func CountErrors(diags []*ast.Diagnostic) int {
	count := 0
	for _, d := range diags {
		if DiagnosticCategory(shimast.Diagnostic_Category(d)) == CategoryError {
			count++
		}
	}
	return count
}

// FilesWithSyntaxErrors returns source file paths that have syntactic diagnostics.
func FilesWithSyntaxErrors(diags []*ast.Diagnostic) map[string]bool {
	files := make(map[string]bool)
	for _, d := range diags {
		if d.File() != nil {
			files[d.File().FileName()] = true
		}
	}
	return files
}

// relativePath converts an absolute path to relative if possible.
func relativePath(absPath string, cwd string) string {
	if cwd == "" {
		return absPath
	}
	rel, err := filepath.Rel(cwd, absPath)
	if err != nil {
		return absPath
	}
	return rel
}
