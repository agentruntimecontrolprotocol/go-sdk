---
title: Agent Versions
sdk: go
spec_sections: [§7.5, §12]
order: 5
kind: feature
---

# Agent Versions

**Negotiation flag:** `agent_versions`. Agents may register under
`name@version`. Bare names resolve to the configured default;
mismatched pins return `AGENT_VERSION_NOT_AVAILABLE`.

```go
srv.RegisterAgentVersion("code-refactor", "1.0.0", v1Fn)
srv.RegisterAgentVersion("code-refactor", "2.0.0", v2Fn)
srv.SetDefaultAgentVersion("code-refactor", "2.0.0")
```

## See also

- [examples/agent-versions](../../examples/agent-versions)
