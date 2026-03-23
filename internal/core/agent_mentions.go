package core

import "strings"

// parseAgentMention checks if the prompt starts with @agentname and returns the
// agent name and remaining text. Returns ("", prompt) if no mention found.
func parseAgentMention(prompt string) (string, string) {
	trimmed := strings.TrimSpace(prompt)
	if !strings.HasPrefix(trimmed, "@") {
		return "", prompt
	}
	parts := strings.SplitN(trimmed[1:], " ", 2)
	if len(parts) == 0 || parts[0] == "" {
		return "", prompt
	}
	rest := ""
	if len(parts) == 2 {
		rest = strings.TrimSpace(parts[1])
	}
	return parts[0], rest
}

// routeToStandbyAgent sends a message to a named standby or running agent.
// Matches by name (case-insensitive), normalized name, or ID.
// Returns true if the message was routed successfully.
func (o *Orchestrator) routeToStandbyAgent(agentName, message string) bool {
	if o.agentManager == nil || message == "" {
		return false
	}
	lower := strings.ToLower(agentName)
	for _, a := range o.agentManager.List() {
		if a.Status != AgentStandby && a.Status != AgentRunning {
			continue
		}
		nameLower := strings.ToLower(a.Name)
		nameNorm := strings.ReplaceAll(nameLower, " ", "_")
		nameHyphen := strings.ReplaceAll(nameLower, " ", "-")
		idLower := strings.ToLower(a.ID)
		if lower == nameLower || lower == nameNorm || lower == nameHyphen || lower == idLower {
			_ = o.agentManager.SendMessage("user", a.ID, message)
			return true
		}
	}
	return false
}
