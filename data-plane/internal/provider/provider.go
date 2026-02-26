package provider

import (
	"context"
	"fmt"
	"strings"
)

type CompletionRequest struct {
	ModelAlias string
	Prompt     string
}

type CompletionResponse struct {
	Content          string
	PromptTokens     int
	CompletionTokens int
}

type StubProvider struct{}

func NewStubProvider() *StubProvider {
	return &StubProvider{}
}

func (p *StubProvider) Generate(_ context.Context, vendorType, vendorModel string, req CompletionRequest) (CompletionResponse, error) {
	if req.Prompt == "" {
		return CompletionResponse{}, fmt.Errorf("empty prompt")
	}
	text := fmt.Sprintf("[%s/%s] %s", vendorType, vendorModel, req.Prompt)
	return CompletionResponse{
		Content:          "ZSGate response: " + text,
		PromptTokens:     max(1, len(strings.Fields(req.Prompt))),
		CompletionTokens: max(1, len(strings.Fields(text))/2),
	}, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
