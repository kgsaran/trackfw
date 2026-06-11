package ai

import "context"

// FakeClient é um cliente de IA para uso em testes.
type FakeClient struct{ Response string }

func (f *FakeClient) Generate(_ context.Context, _ string) (string, error) {
	return f.Response, nil
}
