package parser

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeObservabilityConfigsPreservesImportPolicies(t *testing.T) {
	t.Parallel()

	configs := []string{
		`{"otlp":{"endpoint":"https://first.example/otlp","if-missing":"warn","resource-attributes":{"shared":"first","first":"value"}}}`,
		`{"otlp":{"endpoint":"https://second.example/otlp","if-missing":"ignore","resource-attributes":{"shared":"second","second":"value"}}}`,
	}

	mergedJSON := mergeObservabilityConfigs(configs)
	require.NotEmpty(t, mergedJSON)

	var merged map[string]any
	require.NoError(t, json.Unmarshal([]byte(mergedJSON), &merged))
	otlp := merged["otlp"].(map[string]any)
	assert.Equal(t, "warn", otlp["if-missing"])
	assert.Equal(t, map[string]any{
		"shared": "first",
		"first":  "value",
		"second": "value",
	}, otlp["resource-attributes"])
}

func TestMergeObservabilityConfigsRetainsPolicyWithoutEndpoint(t *testing.T) {
	t.Parallel()

	mergedJSON := mergeObservabilityConfigs([]string{`{"otlp":{"if-missing":"warn"}}`})
	require.NotEmpty(t, mergedJSON)

	var merged map[string]any
	require.NoError(t, json.Unmarshal([]byte(mergedJSON), &merged))
	otlp := merged["otlp"].(map[string]any)
	assert.Equal(t, "warn", otlp["if-missing"])
}
