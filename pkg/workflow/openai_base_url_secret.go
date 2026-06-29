package workflow

import "fmt"

const openAIAPIProxyProvider = "openai"

func extractAPITargetBaseURLSecret(workflowData *WorkflowData, provider string) string {
	if workflowData == nil || workflowData.SandboxConfig == nil || workflowData.SandboxConfig.Agent == nil {
		return ""
	}
	targets := workflowData.SandboxConfig.Agent.Targets
	if targets == nil {
		return ""
	}
	target, ok := targets[provider]
	if !ok || target == nil {
		return ""
	}
	return target.BaseURLSecret
}

func collectAPITargetBaseURLSecretNames(workflowData *WorkflowData) []string {
	secretName := extractAPITargetBaseURLSecret(workflowData, openAIAPIProxyProvider)
	if secretName == "" {
		return nil
	}
	return []string{secretName}
}

func cloneAgentAPITargets(workflowData *WorkflowData) map[string]*AgentAPIProxyTargetConfig {
	agentConfig := getAgentConfig(workflowData)
	if agentConfig == nil || len(agentConfig.Targets) == 0 {
		return nil
	}

	targets := make(map[string]*AgentAPIProxyTargetConfig, len(agentConfig.Targets))
	for provider, target := range agentConfig.Targets {
		if target == nil {
			continue
		}
		targetCopy := *target
		targets[provider] = &targetCopy
	}
	if len(targets) == 0 {
		return nil
	}
	return targets
}

func applyAPITargetBaseURLSecretEnv(env map[string]string, workflowData *WorkflowData) {
	if env == nil {
		return
	}
	if !isFirewallEnabled(workflowData) {
		return
	}
	for _, secretName := range collectAPITargetBaseURLSecretNames(workflowData) {
		env[secretName] = fmt.Sprintf("${{ secrets.%s }}", secretName)
	}
}

func buildOpenAIBaseURLSecretConfigPatchScript(workflowData *WorkflowData) string {
	secretName := extractAPITargetBaseURLSecret(workflowData, openAIAPIProxyProvider)
	if secretName == "" {
		return ""
	}

	return fmt.Sprintf(`python3 - <<'PY'
import json
import os
import urllib.parse

secret_name = %q
config_path = os.path.expandvars(%q)
endpoint = os.environ.get(secret_name, "").strip().rstrip("/")
if not endpoint:
    raise SystemExit(f"{secret_name} is configured but the GitHub Actions secret is empty")

parsed = urllib.parse.urlsplit(endpoint)
if parsed.scheme not in ("http", "https"):
    raise SystemExit(f"{secret_name} must contain an http or https URL")
if parsed.username or parsed.password:
    raise SystemExit(f"{secret_name} must not include embedded credentials")
try:
    host = parsed.hostname
    port = parsed.port
except ValueError:
    raise SystemExit(f"{secret_name} must include a valid host and port")
if not host:
    raise SystemExit(f"{secret_name} must include a valid host")
if parsed.query or parsed.fragment:
    raise SystemExit(f"{secret_name} must not include a query string or fragment")

target_host = parsed.netloc
base_path = parsed.path.rstrip("/")
if base_path == "/openai/v1":
    base_path = "/backend-api/codex"

mask_values = [endpoint, target_host, host]
if port:
    mask_values.append(f"{host}:{port}")
for value in dict.fromkeys(v for v in mask_values if v):
    print(f"::add-mask::{value}")

with open(config_path, "r", encoding="utf-8") as handle:
    config = json.load(handle)

network = config.setdefault("network", {})
allow_domains = network.setdefault("allowDomains", [])
if host not in allow_domains:
    allow_domains.append(host)

api_proxy = config.setdefault("apiProxy", {})
api_proxy["enabled"] = True
targets = api_proxy.setdefault("targets", {})
openai = targets.setdefault("openai", {})
openai["host"] = target_host
if base_path:
    openai["basePath"] = base_path
else:
    openai.pop("basePath", None)

with open(config_path, "w", encoding="utf-8") as handle:
    json.dump(config, handle, ensure_ascii=False, separators=(",", ":"))
    handle.write("\n")
PY`, secretName, awfConfigRuntimePathExpr)
}
