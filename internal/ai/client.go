package ai

import (
	"context"
	"fmt"
)

// Client é a interface para geração de texto via IA.
type Client interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// NewClient cria um Client para o provider configurado.
func NewClient(provider, model, apiKey string) (Client, error) {
	switch provider {
	case "anthropic":
		return newAnthropicClient(model, apiKey), nil
	case "openai":
		return newOpenAIClient(model, apiKey), nil
	default:
		return nil, fmt.Errorf("unknown AI provider: %q", provider)
	}
}
