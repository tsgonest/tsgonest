package rewrite

import (
	"github.com/microsoft/typescript-go/shim/ast"
	"github.com/microsoft/typescript-go/shim/core"
	shimparser "github.com/microsoft/typescript-go/shim/parser"
	shimscanner "github.com/microsoft/typescript-go/shim/scanner"
)

// MethodLoc holds AST-derived positions for a class method.
type MethodLoc struct {
	MethodNamePos  int         // byte offset of method name identifier (after trivia)
	BodyOpenBrace  int         // byte offset of '{' opening the method body
	BodyCloseBrace int         // byte offset of '}' closing the method body
	IsAsync        bool        // has `async` modifier
	Parameters     []string    // parameter names in order
	Returns        []ReturnLoc // top-level return statements (not inside nested functions)
}

// ReturnLoc holds positions for a top-level return statement.
type ReturnLoc struct {
	ReturnKeywordPos int  // byte offset of 'r' in 'return'
	ExprStart        int  // byte offset of expression start (-1 for bare return)
	ExprEnd          int  // byte offset past expression end (-1 for bare return)
	StmtEnd          int  // End() of the entire return statement node
	HasSemicolon     bool // whether terminated by ';' (vs ASI)
}

// DecorateCallLoc holds position info for a __decorate() call statement.
type DecorateCallLoc struct {
	IsClassLevel     bool   // true = ClassName = __decorate([...], ClassName)
	ClassName        string // the class being decorated
	MethodName       string // empty for class-level decorations
	ArrayOpenBracket int    // byte offset of '[' in __decorate([
	StmtEnd          int    // byte offset of End() of the statement
}

// ClassLoc holds position info for a class.
type ClassLoc struct {
	Methods map[string]*MethodLoc
}

// JSLocations holds all AST-derived positions for a JS file.
type JSLocations struct {
	Classes       map[string]*ClassLoc
	DecorateCalls []DecorateCallLoc
}

// LocateJS parses a JS string and extracts structural positions for classes,
// methods, return statements, and __decorate calls.
func LocateJS(text string) *JSLocations {
	sf := shimparser.ParseSourceFile(
		ast.SourceFileParseOptions{FileName: "/input.js"},
		text,
		core.ScriptKindJS,
	)

	locs := &JSLocations{
		Classes: make(map[string]*ClassLoc),
	}

	for _, stmt := range sf.Statements.Nodes {
		switch stmt.Kind {
		case ast.KindClassDeclaration:
			extractClassDecl(text, stmt, locs)
		case ast.KindVariableStatement:
			// Handle: let ClassName = class ClassName { ... }
			// tsgo emits this pattern instead of plain class declarations.
			extractClassFromVarStmt(text, stmt, locs)
		case ast.KindExpressionStatement:
			extractDecorateCall(text, stmt, locs)
		}
	}

	return locs
}

// extractClassDecl processes a ClassDeclaration node and populates locs.Classes.
func extractClassDecl(text string, node *ast.Node, locs *JSLocations) {
	nameNode := node.Name()
	if nameNode == nil {
		return
	}
	className := nameNode.Text()
	if className == "" {
		return
	}
	extractClassMembers(text, className, node.Members(), locs)
}

// extractClassFromVarStmt handles: let ClassName = class ClassName { ... }
// This is the pattern tsgo emits for class declarations.
func extractClassFromVarStmt(text string, stmt *ast.Node, locs *JSLocations) {
	vs := stmt.AsVariableStatement()
	if vs.DeclarationList == nil {
		return
	}
	vdl := vs.DeclarationList.AsVariableDeclarationList()
	if vdl.Declarations == nil {
		return
	}
	for _, decl := range vdl.Declarations.Nodes {
		if decl.Kind != ast.KindVariableDeclaration {
			continue
		}
		vd := decl.AsVariableDeclaration()
		if vd.Initializer == nil || vd.Initializer.Kind != ast.KindClassExpression {
			continue
		}
		// Use the variable name as the class name (matches __decorate references)
		varName := decl.Name()
		if varName == nil {
			continue
		}
		className := varName.Text()
		if className == "" {
			continue
		}
		extractClassMembers(text, className, vd.Initializer.Members(), locs)
	}
}

// extractClassMembers extracts methods from a class's member list.
func extractClassMembers(text string, className string, members []*ast.Node, locs *JSLocations) {
	cls := &ClassLoc{
		Methods: make(map[string]*MethodLoc),
	}
	for _, member := range members {
		if member.Kind == ast.KindMethodDeclaration {
			extractMethod(text, member, cls)
		}
	}
	locs.Classes[className] = cls
}

// extractMethod processes a MethodDeclaration node and adds it to cls.Methods.
func extractMethod(text string, node *ast.Node, cls *ClassLoc) {
	method := node.AsMethodDeclaration()
	nameNode := method.Name()
	if nameNode == nil {
		return
	}
	methodName := shimscanner.GetTextOfNodeFromSourceText(text, nameNode.AsNode(), false)
	if methodName == "" {
		return
	}

	body := method.Body
	if body == nil {
		return
	}

	// Get the actual token positions (skip leading trivia/whitespace)
	namePos := shimscanner.SkipTrivia(text, nameNode.Pos())

	bodyNode := body.AsNode()
	// Body is a Block: find open and close brace positions
	openBrace := shimscanner.SkipTrivia(text, bodyNode.Pos())
	closeBrace := shimscanner.SkipTrivia(text, bodyNode.End()-1)
	// Verify we actually hit braces
	if openBrace < len(text) && text[openBrace] != '{' {
		// Fallback: scan forward from bodyNode.Pos()
		for i := bodyNode.Pos(); i < bodyNode.End(); i++ {
			if text[i] == '{' {
				openBrace = i
				break
			}
		}
	}
	if closeBrace < len(text) && text[closeBrace] != '}' {
		// The close brace should be the last character before End()
		closeBrace = bodyNode.End() - 1
		for closeBrace >= openBrace && text[closeBrace] != '}' {
			closeBrace--
		}
	}

	isAsync := node.ModifierFlags()&ast.ModifierFlagsAsync != 0

	// Extract parameter names (only simple identifiers, skip destructured patterns)
	var params []string
	if method.Parameters != nil {
		for _, p := range method.Parameters.Nodes {
			pName := p.Name()
			if pName != nil && pName.Kind == ast.KindIdentifier {
				params = append(params, pName.AsIdentifier().Text)
			} else {
				// Destructured or other pattern — emit empty string as placeholder
				params = append(params, "")
			}
		}
	}

	ml := &MethodLoc{
		MethodNamePos:  namePos,
		BodyOpenBrace:  openBrace,
		BodyCloseBrace: closeBrace,
		IsAsync:        isAsync,
		Parameters:     params,
	}

	// Collect top-level return statements using ForEachReturnStatement
	ast.ForEachReturnStatement(bodyNode, func(retNode *ast.Node) bool {
		ret := retNode.AsReturnStatement()
		retKeyPos := shimscanner.SkipTrivia(text, retNode.Pos())

		rl := ReturnLoc{
			ReturnKeywordPos: retKeyPos,
			StmtEnd:          retNode.End(),
		}

		if ret.Expression != nil {
			rl.ExprStart = shimscanner.SkipTrivia(text, ret.Expression.Pos())
			rl.ExprEnd = ret.Expression.End()
		} else {
			rl.ExprStart = -1
			rl.ExprEnd = -1
		}

		// Check for semicolon: if the character just before End() is ';'
		if retNode.End() > 0 && retNode.End() <= len(text) {
			lastChar := text[retNode.End()-1]
			rl.HasSemicolon = lastChar == ';'
		}

		ml.Returns = append(ml.Returns, rl)
		return false // continue visiting
	})

	cls.Methods[methodName] = ml
}

// extractDecorateCall checks if an ExpressionStatement is a __decorate call
// and extracts its position info.
func extractDecorateCall(text string, stmt *ast.Node, locs *JSLocations) {
	expr := stmt.AsExpressionStatement().Expression
	if expr == nil {
		return
	}

	switch expr.Kind {
	case ast.KindCallExpression:
		// Method-level: __decorate([...], ClassName.prototype, "methodName", null);
		call := expr.AsCallExpression()
		if !isDecorateCallee(call.Expression) {
			return
		}
		args := call.Arguments
		if args == nil || len(args.Nodes) < 3 {
			return
		}
		// First arg should be an array literal
		arrArg := args.Nodes[0]
		if arrArg.Kind != ast.KindArrayLiteralExpression {
			return
		}
		// Second arg: ClassName.prototype
		className := extractPrototypeClassName(args.Nodes[1])
		if className == "" {
			return
		}
		// Third arg: "methodName" string literal
		methodArg := args.Nodes[2]
		if methodArg.Kind != ast.KindStringLiteral {
			return
		}
		methodName := methodArg.AsStringLiteral().Text

		bracketPos := shimscanner.SkipTrivia(text, arrArg.Pos())

		locs.DecorateCalls = append(locs.DecorateCalls, DecorateCallLoc{
			IsClassLevel:     false,
			ClassName:        className,
			MethodName:       methodName,
			ArrayOpenBracket: bracketPos,
			StmtEnd:          stmt.End(),
		})

	case ast.KindBinaryExpression:
		// Class-level: ClassName = __decorate([...], ClassName);
		bin := expr.AsBinaryExpression()
		if bin.OperatorToken.Kind != ast.KindEqualsToken {
			return
		}
		// Right side should be __decorate(...)
		if bin.Right.Kind != ast.KindCallExpression {
			return
		}
		call := bin.Right.AsCallExpression()
		if !isDecorateCallee(call.Expression) {
			return
		}
		// Left side is the class name
		className := extractIdentifierText(bin.Left)
		if className == "" {
			return
		}
		args := call.Arguments
		if args == nil || len(args.Nodes) < 1 {
			return
		}
		arrArg := args.Nodes[0]
		if arrArg.Kind != ast.KindArrayLiteralExpression {
			return
		}
		bracketPos := shimscanner.SkipTrivia(text, arrArg.Pos())

		locs.DecorateCalls = append(locs.DecorateCalls, DecorateCallLoc{
			IsClassLevel:     true,
			ClassName:        className,
			ArrayOpenBracket: bracketPos,
			StmtEnd:          stmt.End(),
		})
	}
}

// isDecorateCallee checks if an expression is the identifier `__decorate`.
func isDecorateCallee(expr *ast.Node) bool {
	if expr == nil {
		return false
	}
	if expr.Kind == ast.KindIdentifier {
		return expr.AsIdentifier().Text == "__decorate"
	}
	return false
}

// extractPrototypeClassName extracts "ClassName" from a `ClassName.prototype`
// property access expression.
func extractPrototypeClassName(node *ast.Node) string {
	if node == nil || node.Kind != ast.KindPropertyAccessExpression {
		return ""
	}
	pae := node.AsPropertyAccessExpression()
	propName := pae.Name()
	if propName == nil {
		return ""
	}
	propText := propName.Text()
	if propText != "prototype" {
		return ""
	}
	return extractIdentifierText(pae.Expression)
}

// extractIdentifierText returns the text of an Identifier node, or "" if not an identifier.
func extractIdentifierText(node *ast.Node) string {
	if node == nil || node.Kind != ast.KindIdentifier {
		return ""
	}
	return node.AsIdentifier().Text
}
