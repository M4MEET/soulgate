package policy

// Policy represents the complete policy configuration
type Policy struct {
	Version  string       `yaml:"version"`
	Policies []PolicyRule `yaml:"policies"`
}

// PolicyRule represents a single policy rule
type PolicyRule struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Action      string            `yaml:"action"`             // e.g., "files.read", "files.*"
	Resource    string            `yaml:"resource"`           // e.g., "./**", "../**"
	Decision    Decision          `yaml:"decision"`           // allow, deny, or require_approval
	Priority    int               `yaml:"priority,omitempty"` // Higher priority = evaluated first
	Role        string            `yaml:"role,omitempty"`     // admin, user, agent, plugin
	AgentID     string            `yaml:"agent_id,omitempty"` // scope rule to a specific agent
	Conditions  []PolicyCondition `yaml:"conditions,omitempty"`
}

// PolicyCondition represents a conditional expression for richer rule matching
type PolicyCondition struct {
	Field    string `yaml:"field"`    // context field name, e.g., "metadata.env"
	Operator string `yaml:"operator"` // eq, neq, contains, prefix, suffix
	Value    string `yaml:"value"`    // expected value
}

// Decision represents a policy decision
type Decision string

const (
	DecisionAllow           Decision = "allow"
	DecisionDeny            Decision = "deny"
	DecisionRequireApproval Decision = "require_approval"
)

// Action represents different action types
type Action string

const (
	ActionFilesRead  Action = "files.read"
	ActionFilesWrite Action = "files.write"
	ActionFilesList  Action = "files.list"
	ActionFilesStat  Action = "files.stat"
	ActionFilesAll   Action = "files.*"

	ActionNetRequest Action = "net.request"
	ActionNetAll     Action = "net.*"

	ActionExecCommand Action = "exec.command"
	ActionExecAll     Action = "exec.*"

	ActionAll Action = "*"
)

// PolicyRequest represents a request for policy evaluation
type PolicyRequest struct {
	Action   string                 // The action being requested
	Resource string                 // The resource being accessed
	PluginID string                 // The plugin making the request
	RunID    string                 // The run context
	Role     string                 // Caller role: admin, user, agent, plugin
	AgentID  string                 // Calling agent identifier
	Metadata map[string]interface{} // Additional context
}

// PolicyResult represents the result of a policy evaluation
type PolicyResult struct {
	Decision Decision
	Rule     *PolicyRule // The rule that matched (if any)
	Reason   string      // Human-readable explanation
}

// String returns a string representation of the decision
func (d Decision) String() string {
	return string(d)
}

// IsAllow checks if the decision is allow
func (d Decision) IsAllow() bool {
	return d == DecisionAllow
}

// IsDeny checks if the decision is deny
func (d Decision) IsDeny() bool {
	return d == DecisionDeny
}

// IsRequireApproval checks if the decision requires human approval
func (d Decision) IsRequireApproval() bool {
	return d == DecisionRequireApproval
}
