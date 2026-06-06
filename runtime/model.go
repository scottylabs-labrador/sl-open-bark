package runtime

// ModelConfig holds the model strategy (design 5.5): a default everyday model and a stronger model
// for hard tasks. Both come from the environment, so changing models is a config change, not code.
type ModelConfig struct {
	Default    string // GOOSE_MODEL, e.g. "anthropic/claude-sonnet"
	Escalation string // hard tasks, e.g. "anthropic/claude-opus"
}

// Select chooses the model for a task: a per-recipe override wins; otherwise a hard task escalates
// to the stronger model; otherwise the default.
func (m ModelConfig) Select(recipeModel string, hard bool) string {
	if recipeModel != "" {
		return recipeModel
	}
	if hard && m.Escalation != "" {
		return m.Escalation
	}
	return m.Default
}
