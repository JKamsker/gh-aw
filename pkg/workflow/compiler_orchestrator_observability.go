package workflow

import (
	"encoding/json"

	"github.com/github/gh-aw/pkg/setutil"
)

// mergeImportedObservability merges imported OTLP config into raw frontmatter.
// Values declared by the main workflow take precedence over imported values.
func (c *Compiler) mergeImportedObservability(workflowData *WorkflowData, mergedObservability string) {
	if mergedObservability == "" {
		return
	}
	var importedObs map[string]any
	if err := json.Unmarshal([]byte(mergedObservability), &importedObs); err != nil {
		orchestratorWorkflowLog.Printf("Skipping imported observability merge: invalid JSON: %v", err)
		return
	}

	mainObs := extractRawObservabilityMap(workflowData.RawFrontmatter)
	mergedEndpoints, mainCount, importAdded := mergeRawOTLPEndpoints(mainObs, importedObs)
	mergedAttrs := mergeOTLPStringMaps(
		extractOTLPCustomAttributesFromObsMap(mainObs),
		extractOTLPCustomAttributesFromObsMap(importedObs),
	)
	mergedResourceAttrs := mergeOTLPStringMaps(
		extractOTLPResourceAttributesFromObsMap(mainObs),
		extractOTLPResourceAttributesFromObsMap(importedObs),
	)
	githubApp := extractRawOTLPGitHubAppMap(mainObs)
	if githubApp == nil {
		githubApp = extractRawOTLPGitHubAppMap(importedObs)
	}
	ifMissing := extractRawOTLPStringField(mainObs, "if-missing")
	if ifMissing == "" {
		ifMissing = extractRawOTLPStringField(importedObs, "if-missing")
	}

	applyMergedRawObservability(
		workflowData.RawFrontmatter,
		mergedEndpoints,
		mergedAttrs,
		mergedResourceAttrs,
		githubApp,
		ifMissing,
		mainCount,
		importAdded,
	)
}

func extractRawObservabilityMap(rawFrontmatter map[string]any) map[string]any {
	if rawFrontmatter == nil {
		return nil
	}
	obs, _ := rawFrontmatter["observability"].(map[string]any)
	return obs
}

func extractRawOTLPStringField(observability map[string]any, field string) string {
	if observability == nil {
		return ""
	}
	otlp, _ := observability["otlp"].(map[string]any)
	value, _ := otlp[field].(string)
	return value
}

func mergeRawOTLPEndpoints(mainObs map[string]any, importedObs map[string]any) (mergedEndpoints []any, mainCount int, importAdded int) {
	seen := make(map[string]struct{})
	for _, endpoint := range extractRawOTLPEndpointMaps(mainObs) {
		if url, _ := endpoint["url"].(string); url != "" && !setutil.Contains(seen, url) {
			seen[url] = struct{}{}
			mergedEndpoints = append(mergedEndpoints, endpoint)
		}
	}
	mainCount = len(mergedEndpoints)
	for _, endpoint := range extractRawOTLPEndpointMaps(importedObs) {
		if url, _ := endpoint["url"].(string); url != "" && !setutil.Contains(seen, url) {
			seen[url] = struct{}{}
			mergedEndpoints = append(mergedEndpoints, endpoint)
			importAdded++
		}
	}
	return mergedEndpoints, mainCount, importAdded
}

func applyMergedRawObservability(
	rawFrontmatter map[string]any,
	mergedEndpoints []any,
	mergedAttrs map[string]string,
	mergedResourceAttrs map[string]string,
	githubApp map[string]any,
	ifMissing string,
	mainCount int,
	importAdded int,
) {
	if len(mergedEndpoints) == 0 && len(mergedAttrs) == 0 && len(mergedResourceAttrs) == 0 && githubApp == nil && ifMissing == "" {
		return
	}

	newOTLP := map[string]any{}
	if len(mergedEndpoints) > 0 {
		newOTLP["endpoint"] = mergedEndpoints
	}
	if len(mergedAttrs) > 0 {
		newOTLP["attributes"] = mergedAttrs
	}
	if len(mergedResourceAttrs) > 0 {
		newOTLP["resource-attributes"] = mergedResourceAttrs
	}
	if githubApp != nil {
		newOTLP["github-app"] = githubApp
	}
	if ifMissing != "" {
		newOTLP["if-missing"] = ifMissing
	}

	rawFrontmatter["observability"] = map[string]any{"otlp": newOTLP}
	orchestratorWorkflowLog.Printf("Merged OTLP endpoints into RawFrontmatter: %d from main workflow, %d from imports (%d total)", mainCount, importAdded, len(mergedEndpoints))
	if len(mergedAttrs) > 0 {
		orchestratorWorkflowLog.Printf("Merged %d custom OTLP attributes into RawFrontmatter", len(mergedAttrs))
	}
	if len(mergedResourceAttrs) > 0 {
		orchestratorWorkflowLog.Printf("Merged %d OTLP resource attributes into RawFrontmatter", len(mergedResourceAttrs))
	}
}
