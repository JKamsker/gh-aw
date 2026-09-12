// Package parser provides functions for parsing and processing workflow markdown files.
// import_observability.go implements OTLP endpoint extraction and merging for imported
// workflow observability configurations.
package parser

import (
	"encoding/json"
	"maps"

	"github.com/github/gh-aw/pkg/setutil"
)

// observabilityImportEndpoint is an endpoint entry used during import merging.
// Headers are kept as any (original format: string or map) so that the workflow
// package can later normalise both supported forms correctly.
type observabilityImportEndpoint struct {
	URL     string `json:"url"`
	Headers any    `json:"headers,omitempty"`
}

// extractOTLPEndpointsFromObsMap reads the `otlp.endpoint` field from a raw
// observability map and returns all endpoint entries as observabilityImportEndpoints.
// Supports string, object, and array forms of the endpoint field.
// Top-level `headers` is only applied to the backward-compat string endpoint form.
func extractOTLPEndpointsFromObsMap(obs map[string]any) []observabilityImportEndpoint {
	otlpAny, ok := obs["otlp"]
	if !ok {
		return nil
	}
	otlpMap, ok := otlpAny.(map[string]any)
	if !ok {
		return nil
	}

	endpointRaw := otlpMap["endpoint"]
	headersRaw := otlpMap["headers"] // only applies to the backward-compat string form

	var result []observabilityImportEndpoint
	switch ep := endpointRaw.(type) {
	case string:
		if ep != "" {
			entry := observabilityImportEndpoint{URL: ep}
			if headersRaw != nil {
				entry.Headers = headersRaw
			}
			result = append(result, entry)
		}
	case map[string]any:
		if url, _ := ep["url"].(string); url != "" {
			entry := observabilityImportEndpoint{URL: url}
			if h, hasH := ep["headers"]; hasH {
				entry.Headers = h
			}
			result = append(result, entry)
		}
	case []any:
		for _, item := range ep {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			url, _ := itemMap["url"].(string)
			if url == "" {
				continue
			}
			entry := observabilityImportEndpoint{URL: url}
			if h, hasH := itemMap["headers"]; hasH {
				entry.Headers = h
			}
			result = append(result, entry)
		}
	}
	return result
}

// mergeObservabilityConfigs takes a slice of observability config JSON strings (one per
// import), extracts all OTLP endpoint entries from each (supporting string, object, and
// array forms), deduplicates by URL (first occurrence wins), and returns a single merged
// observability JSON string with all endpoints expressed as an array. Custom and
// resource attributes are merged across imports, and the first if-missing policy
// is retained. The first imported value wins for every overlapping setting.
func mergeObservabilityConfigs(configs []string) string {
	merged := newObservabilityImportMerge()

	for i, cfgJSON := range configs {
		if cfgJSON == "" {
			continue
		}
		var obs map[string]any
		if err := json.Unmarshal([]byte(cfgJSON), &obs); err != nil {
			parserLog.Printf("Failed to unmarshal observability config from import %d during merge: %v", i, err)
			continue
		}
		merged.add(obs)
	}
	return merged.toJSON()
}

type observabilityImportMerge struct {
	seenEndpoints      map[string]struct{}
	endpoints          []observabilityImportEndpoint
	attributes         map[string]string
	resourceAttributes map[string]string
	githubApp          map[string]any
	ifMissing          string
}

func newObservabilityImportMerge() *observabilityImportMerge {
	return &observabilityImportMerge{
		seenEndpoints:      make(map[string]struct{}),
		attributes:         make(map[string]string),
		resourceAttributes: make(map[string]string),
	}
}

func (m *observabilityImportMerge) add(obs map[string]any) {
	for _, endpoint := range extractOTLPEndpointsFromObsMap(obs) {
		if setutil.Contains(m.seenEndpoints, endpoint.URL) {
			continue
		}
		m.seenEndpoints[endpoint.URL] = struct{}{}
		m.endpoints = append(m.endpoints, endpoint)
	}
	mergeFirstWins(m.attributes, extractOTLPAttributesFromObsMap(obs))
	mergeFirstWins(m.resourceAttributes, extractOTLPResourceAttributesFromObsMap(obs))
	if m.githubApp == nil {
		m.githubApp = extractOTLPGitHubAppFromObsMap(obs)
	}
	if m.ifMissing == "" {
		m.ifMissing = extractOTLPStringFromObsMap(obs, "if-missing")
	}
}

func mergeFirstWins(destination map[string]string, source map[string]string) {
	for key, value := range source {
		if _, exists := destination[key]; !exists {
			destination[key] = value
		}
	}
}

func (m *observabilityImportMerge) toJSON() string {
	if len(m.endpoints) == 0 && len(m.attributes) == 0 && len(m.resourceAttributes) == 0 && m.githubApp == nil && m.ifMissing == "" {
		return ""
	}

	// Produce a merged config with the endpoint field as an array so that the
	// workflow package's collectAllOTLPEndpoints handles it uniformly.  Include
	// any merged custom attributes so the orchestrator can propagate them.
	otlpMap := map[string]any{}
	if len(m.endpoints) > 0 {
		otlpMap["endpoint"] = m.endpoints
	}
	if len(m.attributes) > 0 {
		otlpMap["attributes"] = m.attributes
	}
	if len(m.resourceAttributes) > 0 {
		otlpMap["resource-attributes"] = m.resourceAttributes
	}
	if m.githubApp != nil {
		otlpMap["github-app"] = m.githubApp
	}
	if m.ifMissing != "" {
		otlpMap["if-missing"] = m.ifMissing
	}
	merged := map[string]any{"otlp": otlpMap}
	b, err := json.Marshal(merged)
	if err != nil {
		parserLog.Printf("Failed to marshal %d merged OTLP endpoints: %v", len(m.endpoints), err)
		return ""
	}
	return string(b)
}

func extractOTLPGitHubAppFromObsMap(obs map[string]any) map[string]any {
	if obs == nil {
		return nil
	}
	otlpAny, ok := obs["otlp"]
	if !ok {
		return nil
	}
	otlpMap, ok := otlpAny.(map[string]any)
	if !ok {
		return nil
	}
	githubAppAny, ok := otlpMap["github-app"]
	if !ok {
		return nil
	}
	githubAppMap, ok := githubAppAny.(map[string]any)
	if !ok {
		return nil
	}
	copyMap := make(map[string]any, len(githubAppMap))
	maps.Copy(copyMap, githubAppMap)
	return copyMap
}

// extractOTLPAttributesFromObsMap reads the custom OTLP attributes map from a
// raw observability section (as parsed from an import's frontmatter).  Only
// string values are accepted; non-string values are silently ignored.
// Returns nil when the field is absent or empty.
//
// Note: this intentionally duplicates the logic of
// workflow.extractOTLPCustomAttributesFromObsMap.  The parser package must not
// import the workflow package (circular-dependency risk), so the helper lives
// here as a local copy.  Both implementations must stay in sync.
func extractOTLPAttributesFromObsMap(obs map[string]any) map[string]string {
	return extractOTLPStringMapFromObsMap(obs, "attributes")
}

func extractOTLPResourceAttributesFromObsMap(obs map[string]any) map[string]string {
	return extractOTLPStringMapFromObsMap(obs, "resource-attributes")
}

func extractOTLPStringFromObsMap(obs map[string]any, field string) string {
	if obs == nil {
		return ""
	}
	otlpMap, _ := obs["otlp"].(map[string]any)
	value, _ := otlpMap[field].(string)
	return value
}

func extractOTLPStringMapFromObsMap(obs map[string]any, field string) map[string]string {
	if obs == nil {
		return nil
	}
	otlpMap, _ := obs["otlp"].(map[string]any)
	attrsMap, ok := otlpMap[field].(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]string, len(attrsMap))
	for k, v := range attrsMap {
		if s, ok := v.(string); ok && k != "" {
			result[k] = s
		}
	}
	return result
}
