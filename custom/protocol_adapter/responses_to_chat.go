// Copyright (C) 2023-2026 QuantumNous
// SPDX-License-Identifier: AGPL-3.0-or-later

package protocol_adapter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
)

// ResponsesRequestToChatCompletionsRequest converts an OpenAI Responses API request
// into an OpenAI Chat Completions API request. This is the reverse of
// ChatCompletionsRequestToResponsesRequest and is needed to bridge Codex CLI
// requests through standard OpenAI-compatible channels.
func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	if req.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	chatReq := &dto.GeneralOpenAIRequest{
		Model:       req.Model,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Store:       req.Store,
		Metadata:    req.Metadata,
	}

	if req.MaxOutputTokens != nil {
		chatReq.MaxCompletionTokens = lo.ToPtr(*req.MaxOutputTokens)
	}

	// Convert instructions → system message
	if len(req.Instructions) > 0 {
		var instructionsStr string
		if err := common.Unmarshal(req.Instructions, &instructionsStr); err == nil {
			instructionsStr = strings.TrimSpace(instructionsStr)
			if instructionsStr != "" {
				chatReq.Messages = append(chatReq.Messages, dto.Message{
					Role: "system",
				})
				chatReq.Messages[len(chatReq.Messages)-1].SetStringContent(instructionsStr)
			}
		}
	}

	// Convert input → messages
	if len(req.Input) > 0 {
		messages, err := convertResponsesInputToMessages(req.Input)
		if err != nil {
			return nil, fmt.Errorf("failed to convert input: %w", err)
		}
		chatReq.Messages = append(chatReq.Messages, messages...)
	}

	// Convert tools
	if len(req.Tools) > 0 {
		var tools []map[string]any
		if err := common.Unmarshal(req.Tools, &tools); err == nil {
			chatTools := make([]dto.ToolCallRequest, 0, len(tools))
			for _, tool := range tools {
				toolType, _ := tool["type"].(string)
				if toolType == "function" {
					name, _ := tool["name"].(string)
					description, _ := tool["description"].(string)
					parameters := tool["parameters"]
					chatTools = append(chatTools, dto.ToolCallRequest{
						Type: "function",
						Function: dto.FunctionRequest{
							Name:        name,
							Description: description,
							Parameters:  parameters,
						},
					})
				}
			}
			chatReq.Tools = chatTools
		}
	}

	// Convert tool_choice
	if len(req.ToolChoice) > 0 {
		chatReq.ToolChoice = convertResponsesToolChoice(req.ToolChoice)
	}

	// Convert reasoning effort
	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		chatReq.ReasoningEffort = req.Reasoning.Effort
	}

	// Convert text (response format)
	if len(req.Text) > 0 {
		chatReq.ResponseFormat = convertResponsesTextToResponseFormat(req.Text)
	}

	// Convert user
	if len(req.User) > 0 {
		chatReq.User = req.User
	}

	// Convert stream_options
	if req.StreamOptions != nil {
		chatReq.StreamOptions = req.StreamOptions
	}

	// If no messages were added, add a minimal user message
	if len(chatReq.Messages) == 0 {
		chatReq.Messages = append(chatReq.Messages, dto.Message{
			Role: "user",
		})
		chatReq.Messages[0].SetStringContent("hi")
	}

	return chatReq, nil
}

// convertResponsesInputToMessages converts the Responses API input field to Chat Completions messages.
// The input field can be:
// - A string (single user message)
// - An array of input items (message objects, function_call, function_call_output, etc.)
func convertResponsesInputToMessages(input json.RawMessage) ([]dto.Message, error) {
	if len(input) == 0 {
		return nil, nil
	}

	// Try string first
	var inputStr string
	if err := common.Unmarshal(input, &inputStr); err == nil {
		msg := dto.Message{Role: "user"}
		msg.SetStringContent(inputStr)
		return []dto.Message{msg}, nil
	}

	// Try array of input items
	var inputItems []map[string]any
	if err := common.Unmarshal(input, &inputItems); err != nil {
		return nil, fmt.Errorf("input must be a string or array, got: %s", string(input[:min(len(input), 50)]))
	}

	var messages []dto.Message
	var pendingToolCalls []dto.ToolCallResponse // collect tool calls for the previous assistant message

	for _, item := range inputItems {
		itemType, _ := item["type"].(string)

		switch itemType {
		case "function_call":
			// Function calls belong to the previous assistant message
			callID, _ := item["call_id"].(string)
			name, _ := item["name"].(string)
			args, _ := item["arguments"].(string)
			pendingToolCalls = append(pendingToolCalls, dto.ToolCallResponse{
				ID:   callID,
				Type: "function",
				Function: dto.FunctionResponse{
					Name:      name,
					Arguments: args,
				},
			})

		case "function_call_output":
			// Flush any pending tool calls into an assistant message first
			if len(pendingToolCalls) > 0 {
				assistantMsg := dto.Message{Role: "assistant"}
				assistantMsg.SetToolCalls(pendingToolCalls)
				assistantMsg.SetStringContent("")
				messages = append(messages, assistantMsg)
				pendingToolCalls = nil
			}

			callID, _ := item["call_id"].(string)
			output, _ := item["output"].(string)
			toolMsg := dto.Message{
				Role:       "tool",
				ToolCallId: callID,
			}
			toolMsg.SetStringContent(output)
			messages = append(messages, toolMsg)

		default:
			// Flush pending tool calls before adding a new non-tool message
			if len(pendingToolCalls) > 0 {
				assistantMsg := dto.Message{Role: "assistant"}
				assistantMsg.SetToolCalls(pendingToolCalls)
				assistantMsg.SetStringContent("")
				messages = append(messages, assistantMsg)
				pendingToolCalls = nil
			}

			role, _ := item["role"].(string)
			if role == "" {
				role = "user"
			}
			// Responses API 'developer' role → Chat Completions 'system' role
			// Codex CLI sends developer messages for system instructions
			if role == "developer" {
				role = "system"
			}

			msg := dto.Message{Role: role}
			content := item["content"]

			switch c := content.(type) {
			case string:
				msg.SetStringContent(c)
			case []interface{}:
				// Array of content parts
				contentParts := make([]dto.MediaContent, 0, len(c))
				for _, part := range c {
					partMap, ok := part.(map[string]any)
					if !ok {
						continue
					}
					partType, _ := partMap["type"].(string)
					switch partType {
					case "input_text", "output_text":
						text, _ := partMap["text"].(string)
						contentParts = append(contentParts, dto.MediaContent{
							Type: dto.ContentTypeText,
							Text: text,
						})
					case "input_image":
						imageURL, _ := partMap["image_url"].(string)
						if imageURL == "" {
							if urlMap, ok := partMap["image_url"].(map[string]any); ok {
								imageURL, _ = urlMap["url"].(string)
							}
						}
						contentParts = append(contentParts, dto.MediaContent{
							Type: dto.ContentTypeImageURL,
							ImageUrl: &dto.MessageImageUrl{
								Url: imageURL,
							},
						})
					default:
						// Pass through as text
						text, _ := partMap["text"].(string)
						if text != "" {
							contentParts = append(contentParts, dto.MediaContent{
								Type: dto.ContentTypeText,
								Text: text,
							})
						}
					}
				}
				msg.Content = contentParts
			default:
				if c != nil {
					msg.SetStringContent(fmt.Sprintf("%v", c))
				} else {
					msg.SetStringContent("")
				}
			}

			messages = append(messages, msg)
		}
	}

	// Flush any remaining pending tool calls
	if len(pendingToolCalls) > 0 {
		assistantMsg := dto.Message{Role: "assistant"}
		assistantMsg.SetToolCalls(pendingToolCalls)
		assistantMsg.SetStringContent("")
		messages = append(messages, assistantMsg)
	}

	return messages, nil
}

// convertResponsesToolChoice converts Responses API tool_choice to Chat Completions tool_choice.
func convertResponsesToolChoice(toolChoice json.RawMessage) any {
	if len(toolChoice) == 0 {
		return nil
	}

	// Try string value first (e.g. "auto", "none", "required")
	var strVal string
	if err := common.Unmarshal(toolChoice, &strVal); err == nil {
		return strVal
	}

	// Try object
	var obj map[string]any
	if err := common.Unmarshal(toolChoice, &obj); err != nil {
		return nil
	}

	objType, _ := obj["type"].(string)
	switch objType {
	case "function":
		// Responses: {"type":"function","name":"..."} → Chat: {"type":"function","function":{"name":"..."}}
		name, _ := obj["name"].(string)
		if name != "" {
			return map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": name,
				},
			}
		}
	}

	return toolChoice
}

// convertResponsesTextToResponseFormat converts Responses API text.format to Chat Completions response_format.
func convertResponsesTextToResponseFormat(text json.RawMessage) *dto.ResponseFormat {
	if len(text) == 0 {
		return nil
	}

	var textObj map[string]any
	if err := common.Unmarshal(text, &textObj); err != nil {
		return nil
	}

	format, _ := textObj["format"].(map[string]any)
	if format == nil {
		return nil
	}

	formatType, _ := format["type"].(string)
	if formatType == "" {
		return nil
	}

	rf := &dto.ResponseFormat{
		Type: formatType,
	}

	if formatType == "json_schema" {
		if schema, ok := format["schema"]; ok {
			if b, err := common.Marshal(schema); err == nil {
				rf.JsonSchema = b
			}
		}
		if name, ok := format["name"].(string); ok {
			if rf.JsonSchema == nil {
				rf.JsonSchema = json.RawMessage(`{}`)
			}
			var schemaMap map[string]any
			if err := common.Unmarshal(rf.JsonSchema, &schemaMap); err == nil {
				schemaMap["name"] = name
				if b, err := common.Marshal(schemaMap); err == nil {
					rf.JsonSchema = b
				}
			}
		}
	}

	return rf
}
