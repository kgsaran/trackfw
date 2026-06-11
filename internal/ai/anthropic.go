package ai

import (
	"context"
	"fmt"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const defaultAnthropicModel = "claude-haiku-4-5-20251001"

type anthropicClient struct {
	model  string
	apiKey string
}

func newAnthropicClient(model, apiKey string) Client {
	if model == "" {
		model = defaultAnthropicModel
	}
	return &anthropicClient{model: model, apiKey: apiKey}
}

func (c *anthropicClient) Generate(ctx context.Context, prompt string) (string, error) {
	var opts []option.RequestOption
	if c.apiKey != "" {
		opts = append(opts, option.WithAPIKey(c.apiKey))
	}

	client := anthropic.NewClient(opts...)

	msg, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 4096,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("anthropic generate: %w", err)
	}

	if len(msg.Content) == 0 {
		return "", fmt.Errorf("anthropic: empty response content")
	}

	return msg.Content[0].Text, nil
}
