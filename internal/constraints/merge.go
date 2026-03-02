package constraints

import "github.com/tsgonest/tsgonest/internal/metadata"

// Merge merges src constraints into dst. src values take precedence
// (override dst values when both are set). This is the single authoritative
// merge implementation — it covers ALL fields including Errors.
func Merge(dst, src *metadata.Constraints) {
	if src.Minimum != nil {
		dst.Minimum = src.Minimum
	}
	if src.Maximum != nil {
		dst.Maximum = src.Maximum
	}
	if src.ExclusiveMinimum != nil {
		dst.ExclusiveMinimum = src.ExclusiveMinimum
	}
	if src.ExclusiveMaximum != nil {
		dst.ExclusiveMaximum = src.ExclusiveMaximum
	}
	if src.MultipleOf != nil {
		dst.MultipleOf = src.MultipleOf
	}
	if src.NumericType != nil {
		dst.NumericType = src.NumericType
	}
	if src.MinLength != nil {
		dst.MinLength = src.MinLength
	}
	if src.MaxLength != nil {
		dst.MaxLength = src.MaxLength
	}
	if src.Pattern != nil {
		dst.Pattern = src.Pattern
	}
	if src.Format != nil {
		dst.Format = src.Format
	}
	if src.StartsWith != nil {
		dst.StartsWith = src.StartsWith
	}
	if src.EndsWith != nil {
		dst.EndsWith = src.EndsWith
	}
	if src.Includes != nil {
		dst.Includes = src.Includes
	}
	if src.Uppercase != nil {
		dst.Uppercase = src.Uppercase
	}
	if src.Lowercase != nil {
		dst.Lowercase = src.Lowercase
	}
	if src.ContentMediaType != nil {
		dst.ContentMediaType = src.ContentMediaType
	}
	if len(src.Transforms) > 0 {
		dst.Transforms = src.Transforms
	}
	if src.MinItems != nil {
		dst.MinItems = src.MinItems
	}
	if src.MaxItems != nil {
		dst.MaxItems = src.MaxItems
	}
	if src.UniqueItems != nil {
		dst.UniqueItems = src.UniqueItems
	}
	if src.Default != nil {
		dst.Default = src.Default
	}
	if src.Coerce != nil {
		dst.Coerce = src.Coerce
	}
	if src.ValidateFn != nil {
		dst.ValidateFn = src.ValidateFn
	}
	if src.ValidateModule != nil {
		dst.ValidateModule = src.ValidateModule
	}
	if src.ErrorMessage != nil {
		dst.ErrorMessage = src.ErrorMessage
	}
	// Errors merge: copy all entries from src, overriding dst where keys collide.
	if len(src.Errors) > 0 {
		if dst.Errors == nil {
			dst.Errors = make(map[string]string, len(src.Errors))
		}
		for k, v := range src.Errors {
			dst.Errors[k] = v
		}
	}
}
