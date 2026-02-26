package analyzer

import (
	"strconv"
	"strings"

	"github.com/microsoft/typescript-go/shim/ast"
)

// classJSDocInfo holds JSDoc metadata extracted from a controller class declaration.
type classJSDocInfo struct {
	// IgnoreOpenAPI is true when the controller should be excluded from OpenAPI generation.
	IgnoreOpenAPI bool
	// Tags are from @tag JSDoc — override the auto-derived controller tag.
	Tags []string
	// Security are from @security JSDoc — inherited by methods without their own @security.
	Security []SecurityRequirement
	// IsPublic is from @public JSDoc — marks all routes as public (no security).
	IsPublic bool
	// Description is from the JSDoc body text or @description tag.
	Description string
}

// extractClassJSDoc parses class-level JSDoc for OpenAPI-relevant metadata.
// Recognized annotations:
//   - @tsgonest-ignore openapi — explicit OpenAPI exclusion
//   - @hidden / @exclude — compatible with NestJS/Swagger convention
//   - @tag <name> — override auto-derived tag for all routes in this controller
//   - @security <scheme> [scopes...] — security requirement inherited by all methods
//   - @public — marks all routes as public (no security requirement)
//   - @description <text> — controller description (not currently used in OpenAPI)
func extractClassJSDoc(classNode *ast.Node) classJSDocInfo {
	var info classJSDocInfo
	if classNode == nil {
		return info
	}
	jsdocs := classNode.JSDoc(nil)
	if len(jsdocs) == 0 {
		return info
	}
	jsdoc := jsdocs[len(jsdocs)-1].AsJSDoc()

	// Extract description from JSDoc body text
	if jsdoc.Comment != nil {
		info.Description = extractNodeListText(jsdoc.Comment)
	}

	if jsdoc.Tags == nil {
		return info
	}
	for _, tagNode := range jsdoc.Tags.Nodes {
		tagName, comment := extractJSDocTagInfo(tagNode)
		switch strings.ToLower(tagName) {
		case "hidden", "exclude":
			info.IgnoreOpenAPI = true
		case "tsgonest-ignore":
			if strings.TrimSpace(strings.ToLower(comment)) == "openapi" {
				info.IgnoreOpenAPI = true
			}
		case "tag":
			t := strings.TrimSpace(comment)
			if t != "" {
				info.Tags = append(info.Tags, t)
			}
		case "security":
			parts := strings.Fields(strings.TrimSpace(comment))
			if len(parts) >= 1 {
				sec := SecurityRequirement{Name: parts[0]}
				if len(parts) > 1 {
					sec.Scopes = parts[1:]
				}
				info.Security = append(info.Security, sec)
			}
		case "public":
			info.IsPublic = true
		case "description":
			info.Description = strings.TrimSpace(comment)
		}
	}
	return info
}

// extractMethodJSDoc extracts OpenAPI-relevant JSDoc tags from a method declaration.
// Returns summary, description, deprecated, hidden, tags, security, error responses, content type,
// operationID override, isPublic, paramDescriptions, extensions, and a set of ignored warning kinds.
func extractMethodJSDoc(node *ast.Node) (summary string, description string, deprecated bool, hidden bool, tags []string, security []SecurityRequirement, errorResponses []ErrorResponse, contentType string, operationIDOverride string, isPublic bool, paramDescriptions map[string]string, extensions map[string]string, ignoreWarnings map[string]bool) {
	if node == nil {
		return
	}

	jsdocs := node.JSDoc(nil)
	if len(jsdocs) == 0 {
		return
	}

	jsdoc := jsdocs[len(jsdocs)-1].AsJSDoc()

	// Extract description from JSDoc comment body
	if jsdoc.Comment != nil {
		description = extractNodeListText(jsdoc.Comment)
	}

	if jsdoc.Tags == nil {
		return
	}

	for _, tagNode := range jsdoc.Tags.Nodes {
		// Handle @param tags specially — they use KindJSDocParameterTag, not KindJSDocTag
		if tagNode.Kind == ast.KindJSDocParameterTag {
			paramTag := tagNode.AsJSDocParameterOrPropertyTag()
			if paramTag != nil && paramTag.Name() != nil {
				name := paramTag.Name().Text()
				desc := ""
				if paramTag.Comment != nil {
					desc = strings.TrimSpace(extractNodeListText(paramTag.Comment))
				}
				if name != "" && desc != "" {
					if paramDescriptions == nil {
						paramDescriptions = make(map[string]string)
					}
					paramDescriptions[name] = desc
				}
			}
			continue
		}

		tagName, comment := extractJSDocTagInfo(tagNode)
		if tagName == "" {
			continue
		}

		switch strings.ToLower(tagName) {
		case "summary":
			summary = strings.TrimSpace(comment)
		case "description":
			// Override body text if explicit @description tag present
			description = strings.TrimSpace(comment)
		case "deprecated":
			deprecated = true
		case "hidden", "exclude":
			hidden = true
		case "tag":
			t := strings.TrimSpace(comment)
			if t != "" {
				tags = append(tags, t)
			}
		case "security":
			parts := strings.Fields(strings.TrimSpace(comment))
			if len(parts) >= 1 {
				sec := SecurityRequirement{Name: parts[0]}
				if len(parts) > 1 {
					sec.Scopes = parts[1:]
				}
				security = append(security, sec)
			}
		case "operationid":
			// Override the auto-generated operationId
			operationIDOverride = strings.TrimSpace(comment)
		case "throws":
			// @throws {400} BadRequestError
			// @throws {404} NotFoundError - The resource was not found
			er := parseThrowsTag(comment)
			if er != nil {
				errorResponses = append(errorResponses, *er)
			}
		case "public":
			isPublic = true
		case "contenttype":
			contentType = strings.TrimSpace(comment)
		case "extension":
			// @extension x-key value
			parts := strings.SplitN(strings.TrimSpace(comment), " ", 2)
			if len(parts) >= 1 && strings.HasPrefix(parts[0], "x-") {
				if extensions == nil {
					extensions = make(map[string]string)
				}
				value := ""
				if len(parts) >= 2 {
					value = strings.TrimSpace(parts[1])
				}
				extensions[parts[0]] = value
			}
		case "tsgonest-ignore":
			// @tsgonest-ignore uses-raw-response
			// Suppresses the specified warning kind.
			kind := strings.TrimSpace(comment)
			if kind != "" {
				if ignoreWarnings == nil {
					ignoreWarnings = make(map[string]bool)
				}
				ignoreWarnings[kind] = true
			}
		}
	}

	return
}

// parseThrowsTag parses a @throws tag comment. Supported formats:
//
//	@throws {400} BadRequestError
//	@throws {404} NotFoundError - The resource was not found
//
// The description (after " - ") is optional. If present, it overrides the
// default status description in the OpenAPI response.
func parseThrowsTag(comment string) *ErrorResponse {
	comment = strings.TrimSpace(comment)
	if !strings.HasPrefix(comment, "{") {
		return nil
	}
	closeBrace := strings.Index(comment, "}")
	if closeBrace < 0 {
		return nil
	}

	statusStr := comment[1:closeBrace]
	statusCode, err := strconv.Atoi(strings.TrimSpace(statusStr))
	if err != nil {
		return nil
	}

	rest := strings.TrimSpace(comment[closeBrace+1:])
	if rest == "" {
		return nil
	}

	// Split on " - " to separate TypeName from description
	typeName := rest
	description := ""
	if idx := strings.Index(rest, " - "); idx >= 0 {
		typeName = strings.TrimSpace(rest[:idx])
		description = strings.TrimSpace(rest[idx+3:])
	}

	if typeName == "" {
		return nil
	}

	return &ErrorResponse{
		StatusCode:  statusCode,
		TypeName:    typeName,
		Description: description,
	}
}
