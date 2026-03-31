package openapi

import "github.com/tsgonest/tsgonest/internal/analyzer"

// FilterOptions controls which controllers/routes appear in an OpenAPI output.
type FilterOptions struct {
	// ControllerInclude keeps only controllers from source files matching these globs.
	ControllerInclude []string
	// ControllerExclude removes controllers from source files matching these globs.
	ControllerExclude []string
	// IncludeTags keeps only routes where at least one tag is in this set.
	IncludeTags []string
	// ExcludeTags removes routes where any tag is in this set.
	ExcludeTags []string
}

// FilterControllers returns a filtered copy of the controllers list.
// File glob filters are applied first, then tag filters (AND composition).
// Controllers with zero routes after filtering are excluded.
func FilterControllers(controllers []analyzer.ControllerInfo, opts FilterOptions) []analyzer.ControllerInfo {
	hasGlobFilter := len(opts.ControllerInclude) > 0 || len(opts.ControllerExclude) > 0
	hasTagFilter := len(opts.IncludeTags) > 0 || len(opts.ExcludeTags) > 0

	if !hasGlobFilter && !hasTagFilter {
		return controllers
	}

	// Build tag lookup sets
	includeSet := toSet(opts.IncludeTags)
	excludeSet := toSet(opts.ExcludeTags)

	var result []analyzer.ControllerInfo
	for _, ctrl := range controllers {
		// Step 1: File glob filter
		if hasGlobFilter {
			if !analyzer.MatchesGlob(ctrl.SourceFile, opts.ControllerInclude, opts.ControllerExclude) {
				continue
			}
		}

		// Step 2: Tag filter on routes
		if hasTagFilter {
			var filteredRoutes []analyzer.Route
			for _, route := range ctrl.Routes {
				if matchesTags(route.Tags, includeSet, excludeSet) {
					filteredRoutes = append(filteredRoutes, route)
				}
			}
			if len(filteredRoutes) == 0 {
				continue
			}
			// Copy controller with filtered routes
			filtered := ctrl
			filtered.Routes = filteredRoutes
			result = append(result, filtered)
		} else {
			result = append(result, ctrl)
		}
	}

	return result
}

// matchesTags checks if a route's tags pass include/exclude filters.
func matchesTags(tags []string, includeSet, excludeSet map[string]bool) bool {
	// ExcludeTags: reject if any tag matches
	if len(excludeSet) > 0 {
		for _, tag := range tags {
			if excludeSet[tag] {
				return false
			}
		}
	}

	// IncludeTags: accept if at least one tag matches
	if len(includeSet) > 0 {
		for _, tag := range tags {
			if includeSet[tag] {
				return true
			}
		}
		return false // no tag matched the include set
	}

	return true
}

func toSet(items []string) map[string]bool {
	if len(items) == 0 {
		return nil
	}
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}
