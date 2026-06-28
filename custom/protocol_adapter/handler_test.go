// Copyright (C) 2023-2026 QuantumNous
// SPDX-License-Identifier: AGPL-3.0-or-later

package protocol_adapter

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func TestResponsesRequestToChatCompletionsRequest_StringInput(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:       "gpt-4o",
		Input:       json.RawMessage(`"Hello, how are you?"`),
		Stream:      common.GetPointer(false),
		Temperature: common.GetPointer(0.7),
	}

	chatReq, err := ResponsesRequestToChatCompletionsRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if chatReq.Model != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %s", chatReq.Model)
	}
	if chatReq.Temperature == nil || *chatReq.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7, got %v", chatReq.Temperature)
	}
	if len(chatReq.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(chatReq.Messages))
	}
	if chatReq.Messages[0].Role != "user" {
		t.Errorf("expected role user, got %s", chatReq.Messages[0].Role)
	}
	if !chatReq.Messages[0].IsStringContent() || chatReq.Messages[0].StringContent() != "Hello, how are you?" {
		t.Errorf("expected content 'Hello, how are you?', got %v", chatReq.Messages[0].Content)
	}
}

func TestResponsesRequestToChatCompletionsRequest_ArrayInput(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-4o",
		Input: json.RawMessage(`[
			{"role": "user", "content": "Hello"},
			{"role": "assistant", "content": "Hi there!"},
			{"role": "user", "content": "How are you?"}
		]`),
		Instructions: json.RawMessage(`"You are a helpful assistant"`),
	}

	chatReq, err := ResponsesRequestToChatCompletionsRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have: system + 3 messages = 4
	if len(chatReq.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(chatReq.Messages))
	}

	// First message should be system (from instructions)
	if chatReq.Messages[0].Role != "system" {
		t.Errorf("expected first message role system, got %s", chatReq.Messages[0].Role)
	}
	if !chatReq.Messages[0].IsStringContent() || chatReq.Messages[0].StringContent() != "You are a helpful assistant" {
		t.Errorf("expected system message content, got %v", chatReq.Messages[0].Content)
	}
}

func TestResponsesRequestToChatCompletionsRequest_ToolCalls(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-4o",
		Input: json.RawMessage(`[
			{"role": "user", "content": "What's the weather?"},
			{"type": "function_call", "call_id": "call_123", "name": "get_weather", "arguments": "{\"city\": \"NYC\"}"},
			{"type": "function_call_output", "call_id": "call_123", "output": "Sunny, 72°F"}
		]`),
	}

	chatReq, err := ResponsesRequestToChatCompletionsRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have: user + assistant (with tool call) + tool = 3 messages
	if len(chatReq.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(chatReq.Messages))
	}

	// Second message should be assistant with tool calls
	if chatReq.Messages[1].Role != "assistant" {
		t.Errorf("expected assistant role, got %s", chatReq.Messages[1].Role)
	}

	// Third message should be tool
	if chatReq.Messages[2].Role != "tool" {
		t.Errorf("expected tool role, got %s", chatReq.Messages[2].Role)
	}
	if chatReq.Messages[2].ToolCallId != "call_123" {
		t.Errorf("expected tool_call_id call_123, got %s", chatReq.Messages[2].ToolCallId)
	}
}

func TestResponsesRequestToChatCompletionsRequest_Tools(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-4o",
		Input: json.RawMessage(`"Use the tools"`),
		Tools: json.RawMessage(`[
			{"type": "function", "name": "get_weather", "description": "Get weather", "parameters": {"type": "object", "properties": {"city": {"type": "string"}}}},
			{"type": "function", "name": "get_time", "description": "Get time"}
		]`),
	}

	chatReq, err := ResponsesRequestToChatCompletionsRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chatReq.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(chatReq.Tools))
	}
	if chatReq.Tools[0].Function.Name != "get_weather" {
		t.Errorf("expected tool name get_weather, got %s", chatReq.Tools[0].Function.Name)
	}
}

func TestConvertChatResponseToResponses(t *testing.T) {
	chatResp := &dto.OpenAITextResponse{
		Id:     "chatcmpl-123",
		Object: "chat.completion",
		Model:  "gpt-4o",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role: "assistant",
				},
				FinishReason: "stop",
			},
		},
	}
	chatResp.Choices[0].Message.SetStringContent("Hello! How can I help you?")

	resp, err := ConvertChatResponseToResponses(chatResp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ID != "chatcmpl-123" {
		t.Errorf("expected ID chatcmpl-123, got %s", resp.ID)
	}
	if resp.Object != "response" {
		t.Errorf("expected object response, got %s", resp.Object)
	}
	if len(resp.Output) != 1 {
		t.Fatalf("expected 1 output, got %d", len(resp.Output))
	}
	if resp.Output[0].Type != "message" {
		t.Errorf("expected output type message, got %s", resp.Output[0].Type)
	}
	if resp.Output[0].Role != "assistant" {
		t.Errorf("expected role assistant, got %s", resp.Output[0].Role)
	}
	if len(resp.Output[0].Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(resp.Output[0].Content))
	}
	if resp.Output[0].Content[0].Type != "output_text" {
		t.Errorf("expected content type output_text, got %s", resp.Output[0].Content[0].Type)
	}
	if resp.Output[0].Content[0].Text != "Hello! How can I help you?" {
		t.Errorf("expected text 'Hello! How can I help you?', got %s", resp.Output[0].Content[0].Text)
	}
}

func TestResponsesRequestToChatCompletionsRequest_NilRequest(t *testing.T) {
	_, err := ResponsesRequestToChatCompletionsRequest(nil)
	if err == nil {
		t.Error("expected error for nil request")
	}
}

func TestResponsesRequestToChatCompletionsRequest_NoModel(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Input: json.RawMessage(`"hello"`),
	}
	_, err := ResponsesRequestToChatCompletionsRequest(req)
	if err == nil {
		t.Error("expected error for empty model")
	}
}
