package runtime

import "os"

// Config is the runtime's environment configuration (design 5.5). The OpenRouter key and gateway
// token come from the environment / Railway secrets, never from code.
type Config struct {
	Provider         string // GOOSE_PROVIDER (default openrouter)
	Model            string // GOOSE_MODEL (default anthropic/claude-sonnet)
	EscalationModel  string // GOOSE_ESCALATION_MODEL (default anthropic/claude-opus)
	OpenRouterAPIKey string // OPENROUTER_API_KEY
	GatewayURL       string // MCP gateway extension endpoint (Streamable HTTP /mcp)
	GatewayToken     string // service token presented to the gateway
	RecipesDir       string // where recipes live (default recipes)
	GooseHintsPath   string // optional .goosehints path
}

// LoadConfig reads the runtime config from the environment, applying defaults.
func LoadConfig() Config {
	return Config{
		Provider:         firstNonEmpty(os.Getenv("GOOSE_PROVIDER"), "openrouter"),
		Model:            firstNonEmpty(os.Getenv("GOOSE_MODEL"), "anthropic/claude-sonnet-4.6"),
		EscalationModel:  firstNonEmpty(os.Getenv("GOOSE_ESCALATION_MODEL"), "anthropic/claude-opus-4.8"),
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
		GatewayURL:       os.Getenv("GATEWAY_MCP_URL"),
		GatewayToken:     os.Getenv("GATEWAY_SERVICE_TOKEN"),
		RecipesDir:       firstNonEmpty(os.Getenv("RECIPES_DIR"), "recipes"),
		GooseHintsPath:   os.Getenv("GOOSEHINTS_PATH"),
	}
}

// Models returns the model strategy from config.
func (c Config) Models() ModelConfig {
	return ModelConfig{Default: c.Model, Escalation: c.EscalationModel}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
