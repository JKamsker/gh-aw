package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeImportedObservabilityPreservesIfMissingPolicy(t *testing.T) {
	t.Parallel()

	workflowData := &WorkflowData{
		RawFrontmatter: map[string]any{},
	}
	compiler := NewCompiler()
	compiler.mergeImportedObservability(
		workflowData,
		`{"otlp":{"endpoint":[{"url":"https://example.com/otlp"}],"if-missing":"warn"}}`,
	)

	otlp := workflowData.RawFrontmatter["observability"].(map[string]any)["otlp"].(map[string]any)
	assert.Equal(t, "warn", otlp["if-missing"])

	compiler.injectOTLPConfig(workflowData)
	assert.Contains(t, workflowData.Env, "GH_AW_OTLP_IF_MISSING: warn")
}

func TestMergeImportedObservabilityMainIfMissingPolicyWins(t *testing.T) {
	t.Parallel()

	workflowData := &WorkflowData{
		RawFrontmatter: map[string]any{
			"observability": map[string]any{
				"otlp": map[string]any{
					"endpoint":   "https://main.example/otlp",
					"if-missing": "error",
				},
			},
		},
	}
	NewCompiler().mergeImportedObservability(
		workflowData,
		`{"otlp":{"endpoint":[{"url":"https://import.example/otlp"}],"if-missing":"warn"}}`,
	)

	otlp := workflowData.RawFrontmatter["observability"].(map[string]any)["otlp"].(map[string]any)
	assert.Equal(t, "error", otlp["if-missing"])
}
