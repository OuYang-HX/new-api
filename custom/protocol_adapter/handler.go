// Copyright (C) 2023-2026 QuantumNous
// SPDX-License-Identifier: AGPL-3.0-or-later

package protocol_adapter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// controllerRelayFn is injected via SetRelayFunc to avoid import cycles.
var controllerRelayFn func(c *gin.Context, relayFormat types.RelayFormat)

// enabledModelsFn is injected via SetEnabledModelsFn to avoid import cycles.
var enabledModelsFn func() []string

// SetRelayFunc injects the controller.Relay function.
func SetRelayFunc(f func(c *gin.Context, relayFormat types.RelayFormat)) {
	controllerRelayFn = f
}

// SetEnabledModelsFn injects the model.GetEnabledModels function.
func SetEnabledModelsFn(f func() []string) {
	enabledModelsFn = f
}

// HandleCodexResponses handles /v1/codex/responses requests.
// It accepts Responses API format (as Codex CLI sends), converts to chat/completions,
// relays through the standard pipeline, and converts the response back to Responses format.
func HandleCodexResponses(c *gin.Context) {
	if controllerRelayFn == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"message": "protocol adapter not initialized", "type": "server_error"},
		})
		return
	}

	// Read the original request body from BodyStorage (set by middleware)
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		writeResponsesError(c, http.StatusBadRequest, fmt.Sprintf("Failed to read request body: %v", err))
		return
	}
	bodyBytes, err := io.ReadAll(storage)
	if err != nil {
		writeResponsesError(c, http.StatusBadRequest, fmt.Sprintf("Failed to read request: %v", err))
		return
	}

	var req dto.OpenAIResponsesRequest
	if err := common.Unmarshal(bodyBytes, &req); err != nil {
		writeResponsesError(c, http.StatusBadRequest, fmt.Sprintf("Failed to parse request: %v", err))
		return
	}

	isStream := req.Stream != nil && *req.Stream

	// Convert Responses request → Chat Completions request
	chatReq, err := ResponsesRequestToChatCompletionsRequest(&req)
	if err != nil {
		writeResponsesError(c, http.StatusBadRequest, fmt.Sprintf("Failed to convert request: %v", err))
		return
	}

	chatJSON, err := common.Marshal(chatReq)
	if err != nil {
		writeResponsesError(c, http.StatusInternalServerError, fmt.Sprintf("Failed to marshal request: %v", err))
		return
	}

	logger.LogDebug(c, "protocol_adapter: codex responses→chat model=%s", chatReq.Model)

	// Replace both c.Request.Body and the cached BodyStorage so that
	// downstream middleware (TokenAuth, Distribute) and controller.Relay
	// all see the converted chat/completions request body.
	c.Request.Body = io.NopCloser(bytes.NewReader(chatJSON))
	c.Request.ContentLength = int64(len(chatJSON))
	c.Request.Header.Set("Content-Type", "application/json")
	// Invalidate the BodyStorage cache so the next GetBodyStorage call
	// re-reads from the new c.Request.Body.
	common.CleanupBodyStorage(c)
	// Pre-populate BodyStorage with the converted body so downstream code
	// that calls GetBodyStorage gets the chat/completions format.
	newStorage, storeErr := common.CreateBodyStorage(chatJSON)
	if storeErr == nil {
		c.Set(common.KeyBodyStorage, newStorage)
	}

	// Set relay_mode so GenRelayInfo uses RelayModeChatCompletions.
	// Without this, Path2RelayMode("/v1/codex/responses") returns
	// RelayModeUnknown and the Custom channel adaptor appends the
	// original URL path to the base URL, producing a wrong upstream URL.
	c.Set("relay_mode", 1) // relayconstant.RelayModeChatCompletions = 1

	// Override the URL path so Custom channel type constructs the correct
	// upstream URL (/v1/chat/completions instead of /v1/codex/responses).
	c.Request.URL.Path = "/v1/chat/completions"

	if isStream {
		handleCodexStream(c, chatReq)
	} else {
		handleCodexNonStream(c)
	}
}

// handleCodexNonStream handles non-streaming Codex responses.
func handleCodexNonStream(c *gin.Context) {
	original := c.Writer
	buf := newBufferedWriter(original)
	c.Writer = buf

	controllerRelayFn(c, types.RelayFormatOpenAI)

	capturedBody := buf.body.Bytes()
	if len(capturedBody) == 0 {
		return
	}

	// Try to parse as chat completions response
	var chatResp dto.OpenAITextResponse
	if err := common.Unmarshal(capturedBody, &chatResp); err != nil {
		forwardBufferedResponse(original, buf)
		return
	}

	if chatResp.Object == "" {
		forwardBufferedResponse(original, buf)
		return
	}

	if oaiErr := chatResp.GetOpenAIError(); oaiErr != nil && oaiErr.Type != "" {
		forwardBufferedResponse(original, buf)
		return
	}

	responsesResp, err := ConvertChatResponseToResponses(&chatResp)
	if err != nil {
		forwardBufferedResponse(original, buf)
		return
	}

	responseData, err := common.Marshal(responsesResp)
	if err != nil {
		forwardBufferedResponse(original, buf)
		return
	}

	// Write converted response directly to the original writer.
	// Since we buffered the entire response, headers haven't been
	// sent to the client yet, so we can safely set the correct
	// Content-Length and write the body.
	original.Header().Set("Content-Type", "application/json")
	original.Header().Set("Content-Length", strconv.Itoa(len(responseData)))
	original.WriteHeader(http.StatusOK)
	_, _ = original.Write(responseData)
}

// handleCodexStream handles streaming Codex responses.
func handleCodexStream(c *gin.Context, chatReq *dto.GeneralOpenAIRequest) {
	interceptor := newResponsesStreamInterceptor(c.Writer, chatReq.Model)
	c.Writer = interceptor
	interceptor.SendStartEvents()

	controllerRelayFn(c, types.RelayFormatOpenAI)

	interceptor.SendFinalEvents()
}

// HandleClaudeMessages handles /v1/claude/messages requests.
// Routes through the standard Claude relay which already handles
// request/response protocol conversion.
func HandleClaudeMessages(c *gin.Context) {
	if controllerRelayFn == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"type":  "error",
			"error": gin.H{"message": "protocol adapter not initialized", "type": "server_error"},
		})
		return
	}

	logger.LogDebug(c, "protocol_adapter: claude messages request")

	// Override the URL path so Custom channel type constructs the correct
	// upstream URL (/v1/chat/completions instead of /v1/claude/messages).
	// The Claude relay handler (ClaudeHelper) will convert the request
	// format internally, but the URL path is used by GetRequestURL.
	c.Request.URL.Path = "/v1/chat/completions"

	controllerRelayFn(c, types.RelayFormatClaude)
}

// ConvertChatResponseToResponses converts a Chat Completions response to Responses API format.
func ConvertChatResponseToResponses(chatResp *dto.OpenAITextResponse) (*dto.OpenAIResponsesResponse, error) {
	if chatResp == nil {
		return nil, fmt.Errorf("response is nil")
	}

	output := make([]dto.ResponsesOutput, 0)

	for _, choice := range chatResp.Choices {
		content := make([]dto.ResponsesOutputContent, 0)

		if choice.Message.IsStringContent() {
			text := choice.Message.StringContent()
			if text != "" {
				content = append(content, dto.ResponsesOutputContent{
					Type: "output_text",
					Text: text,
				})
			}
		} else {
			parts := choice.Message.ParseContent()
			for _, part := range parts {
				if part.Type == dto.ContentTypeText {
					content = append(content, dto.ResponsesOutputContent{
						Type: "output_text",
						Text: part.Text,
					})
				}
			}
		}

		output = append(output, dto.ResponsesOutput{
			Type:    "message",
			ID:      fmt.Sprintf("msg_%s", chatResp.Id),
			Status:  "completed",
			Role:    "assistant",
			Content: content,
		})

		// Handle tool calls
		for _, tc := range choice.Message.ParseToolCalls() {
			output = append(output, dto.ResponsesOutput{
				Type:      "function_call",
				ID:        fmt.Sprintf("fc_%s_%s", chatResp.Id, tc.ID),
				CallId:    tc.ID,
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			})
		}
	}

	usage := chatResp.Usage

	createdAt := 0
	if v, ok := chatResp.Created.(int64); ok {
		createdAt = int(v)
	} else if v, ok := chatResp.Created.(int); ok {
		createdAt = v
	} else if v, ok := chatResp.Created.(float64); ok {
		createdAt = int(v)
	}

	resp := &dto.OpenAIResponsesResponse{
		ID:        chatResp.Id,
		Object:    "response",
		CreatedAt: createdAt,
		Status:    json.RawMessage(`"completed"`),
		Model:     chatResp.Model,
		Output:    output,
		Usage:     &usage,
	}

	return resp, nil
}

// --- Buffered response writer ---

type bufferedWriter struct {
	gin.ResponseWriter
	body       bytes.Buffer
	statusCode int
	written    bool
	size       int
}

func newBufferedWriter(original gin.ResponseWriter) *bufferedWriter {
	return &bufferedWriter{
		ResponseWriter: original,
		statusCode:     http.StatusOK,
	}
}

func (w *bufferedWriter) Write(data []byte) (int, error) {
	n, err := w.body.Write(data)
	w.size += n
	return n, err
}
func (w *bufferedWriter) WriteHeader(code int)  { w.statusCode = code; w.written = true }
func (w *bufferedWriter) WriteHeaderNow()       {} // suppress: don't send headers to client yet
func (w *bufferedWriter) Status() int           { return w.statusCode }
func (w *bufferedWriter) Written() bool         { return w.written }
func (w *bufferedWriter) Size() int             { return w.size }
func (w *bufferedWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }
func (w *bufferedWriter) Flush()                {}
func (w *bufferedWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.ResponseWriter.Hijack()
}
func (w *bufferedWriter) Pusher() http.Pusher {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher
	}
	return nil
}

func forwardBufferedResponse(dst gin.ResponseWriter, src *bufferedWriter) {
	for k, v := range src.Header() {
		dst.Header()[k] = v
	}
	dst.WriteHeader(src.statusCode)
	dst.Write(src.body.Bytes())
}

// --- SSE stream interceptor ---

type responsesStreamInterceptor struct {
	gin.ResponseWriter
	responseID string
	messageID  string
	model      string
	usage      *dto.Usage
	fullText   strings.Builder
	// Tool call tracking for function_call output items
	toolCalls      []dto.ToolCallResponse // accumulated tool calls
	currentToolIdx int                    // index of the tool call currently being streamed
	toolItemStarted bool                  // whether output_item.added was sent for current tool
	hasTextContent  bool                  // whether any text content was received
}

func newResponsesStreamInterceptor(w gin.ResponseWriter, model string) *responsesStreamInterceptor {
	return &responsesStreamInterceptor{
		ResponseWriter: w,
		responseID:     fmt.Sprintf("resp_%s", common.GetRandomString(24)),
		messageID:      fmt.Sprintf("msg_%s", common.GetRandomString(24)),
		model:          model,
		usage:          &dto.Usage{},
	}
}

func (w *responsesStreamInterceptor) Write(data []byte) (int, error) {
	dataStr := string(data)

	if strings.HasPrefix(dataStr, "data: ") {
		jsonStr := strings.TrimPrefix(dataStr, "data: ")
		jsonStr = strings.TrimRight(jsonStr, "\r\n")

		if jsonStr == "[DONE]" {
			return len(data), nil
		}

		var chunk dto.ChatCompletionsStreamResponse
		if err := common.Unmarshal([]byte(jsonStr), &chunk); err != nil {
			return len(data), nil
		}

		w.convertChunkToResponsesEvents(&chunk)
		return len(data), nil
	}

	return len(data), nil
}

func (w *responsesStreamInterceptor) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *responsesStreamInterceptor) SendStartEvents() {
	w.writeSSE("response.created", map[string]any{
		"type": "response.created",
		"response": map[string]any{
			"id": w.responseID, "object": "response", "status": "in_progress",
			"model": w.model, "output": []any{}, "stream": true, "created_at": time.Now().Unix(),
		},
	})

	w.writeSSE("response.in_progress", map[string]any{
		"type": "response.in_progress",
		"response": map[string]any{
			"id": w.responseID, "object": "response", "status": "in_progress", "model": w.model,
		},
	})

	w.writeSSE("response.output_item.added", map[string]any{
		"type": "response.output_item.added", "output_index": 0,
		"item": map[string]any{
			"type": "message", "id": w.messageID, "status": "in_progress", "role": "assistant", "content": []any{},
		},
	})

	w.writeSSE("response.content_part.added", map[string]any{
		"type": "response.content_part.added", "output_index": 0, "content_index": 0,
		"part": map[string]any{"type": "output_text", "text": ""},
	})
}

func (w *responsesStreamInterceptor) SendFinalEvents() {
	text := w.fullText.String()
	output := make([]any, 0)

	// Build the message output item
	if w.hasTextContent || len(w.toolCalls) == 0 {
		w.writeSSE("response.output_text.done", map[string]any{
			"type": "response.output_text.done", "output_index": 0, "content_index": 0, "text": text,
		})

		w.writeSSE("response.content_part.done", map[string]any{
			"type": "response.content_part.done", "output_index": 0, "content_index": 0,
			"part": map[string]any{"type": "output_text", "text": text},
		})

		w.writeSSE("response.output_item.done", map[string]any{
			"type": "response.output_item.done", "output_index": 0,
			"item": map[string]any{
				"type": "message", "id": w.messageID, "status": "completed",
				"role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": text}},
			},
		})

		output = append(output, map[string]any{
			"type": "message", "id": w.messageID, "status": "completed",
			"role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": text}},
		})
	} else {
		// No text content, close the message item as empty
		w.writeSSE("response.output_item.done", map[string]any{
			"type": "response.output_item.done", "output_index": 0,
			"item": map[string]any{
				"type": "message", "id": w.messageID, "status": "completed",
				"role": "assistant", "content": []any{},
			},
		})
	}

	// Close any still-open function_call items and add to output
	for i, tc := range w.toolCalls {
		outputIdx := i + 1 // message is at 0, function_calls start at 1
		if i == w.currentToolIdx && w.toolItemStarted {
			w.writeSSE("response.function_call_arguments.done", map[string]any{
				"type": "function_call", "call_id": tc.ID, "output_index": outputIdx,
			})
			w.writeSSE("response.output_item.done", map[string]any{
				"type": "response.output_item.done", "output_index": outputIdx,
				"item": map[string]any{
					"type": "function_call", "id": fmt.Sprintf("fc_%s", tc.ID),
					"call_id": tc.ID, "name": tc.Function.Name,
					"arguments": tc.Function.Arguments, "status": "completed",
				},
			})
		}
		output = append(output, map[string]any{
			"type": "function_call", "id": fmt.Sprintf("fc_%s", tc.ID),
			"call_id": tc.ID, "name": tc.Function.Name,
			"arguments": tc.Function.Arguments, "status": "completed",
		})
	}

	w.writeSSE("response.completed", map[string]any{
		"type": "response.completed",
		"response": map[string]any{
			"id": w.responseID, "object": "response", "status": "completed", "model": w.model,
			"output": output,
			"usage": map[string]any{
				"input_tokens": w.usage.PromptTokens, "output_tokens": w.usage.CompletionTokens,
				"total_tokens": w.usage.TotalTokens,
			},
		},
	})

	w.ResponseWriter.Write([]byte("data: [DONE]\n\n"))
	w.ResponseWriter.Flush()
}

func (w *responsesStreamInterceptor) convertChunkToResponsesEvents(chunk *dto.ChatCompletionsStreamResponse) {
	for _, choice := range chunk.Choices {
		// Handle text content
		if choice.Delta.Content != nil && *choice.Delta.Content != "" {
			text := *choice.Delta.Content
			w.fullText.WriteString(text)
			w.hasTextContent = true
			w.writeSSE("response.output_text.delta", map[string]any{
				"type": "output_text", "text": text, "output_index": 0, "content_index": 0,
			})
		}

		// Handle reasoning content
		if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
			text := *choice.Delta.ReasoningContent
			w.fullText.WriteString(text)
			w.hasTextContent = true
			w.writeSSE("response.output_text.delta", map[string]any{
				"type": "output_text", "text": text, "output_index": 0, "content_index": 0,
			})
		}

		// Handle tool calls — each tool call is a separate output item in Responses API
		for _, tc := range choice.Delta.ToolCalls {
			tcIdx := 0
			if tc.Index != nil {
				tcIdx = *tc.Index
			}

			// Accumulate or create tool call entry
			for len(w.toolCalls) <= tcIdx {
				w.toolCalls = append(w.toolCalls, dto.ToolCallResponse{
					ID:   tc.ID,
					Type: "function",
					Function: dto.FunctionResponse{
						Name:      tc.Function.Name,
						Arguments: "",
					},
				})
			}

			tcEntry := &w.toolCalls[tcIdx]

			// If this chunk has a name, it's the start of a new function_call item
			if tc.Function.Name != "" {
				tcEntry.Function.Name = tc.Function.Name
				if tc.ID != "" {
					tcEntry.ID = tc.ID
				}
				w.currentToolIdx = tcIdx
				w.toolItemStarted = true

				outputIdx := tcIdx + 1 // message is at index 0
				w.writeSSE("response.output_item.added", map[string]any{
					"type": "response.output_item.added", "output_index": outputIdx,
					"item": map[string]any{
						"type": "function_call", "id": fmt.Sprintf("fc_%s", tcEntry.ID),
						"call_id": tcEntry.ID, "name": tcEntry.Function.Name,
						"arguments": "", "status": "in_progress",
					},
				})

				// If there's also arguments in this chunk, send them
				if tc.Function.Arguments != "" {
					tcEntry.Function.Arguments += tc.Function.Arguments
					w.writeSSE("response.function_call_arguments.delta", map[string]any{
						"type": "function_call", "call_id": tcEntry.ID,
						"output_index": outputIdx, "arguments": tc.Function.Arguments,
					})
				}
			} else if tc.Function.Arguments != "" {
				// Arguments continuation chunk
				tcEntry.Function.Arguments += tc.Function.Arguments
				outputIdx := tcIdx + 1
				w.writeSSE("response.function_call_arguments.delta", map[string]any{
					"type": "function_call", "call_id": tcEntry.ID,
					"output_index": outputIdx, "arguments": tc.Function.Arguments,
				})
			}

			// Handle FinishReason for tool_calls — close the current function_call item
			if choice.FinishReason != nil && *choice.FinishReason == "tool_calls" && w.toolItemStarted {
				outputIdx := w.currentToolIdx + 1
				w.writeSSE("response.function_call_arguments.done", map[string]any{
					"type": "function_call", "call_id": w.toolCalls[w.currentToolIdx].ID,
					"output_index": outputIdx,
				})
				w.writeSSE("response.output_item.done", map[string]any{
					"type": "response.output_item.done", "output_index": outputIdx,
					"item": map[string]any{
						"type": "function_call", "id": fmt.Sprintf("fc_%s", w.toolCalls[w.currentToolIdx].ID),
						"call_id": w.toolCalls[w.currentToolIdx].ID,
						"name": w.toolCalls[w.currentToolIdx].Function.Name,
						"arguments": w.toolCalls[w.currentToolIdx].Function.Arguments,
						"status": "completed",
					},
				})
				w.toolItemStarted = false
			}
		}

		// Handle usage
		if chunk.Usage != nil && chunk.Usage.TotalTokens > 0 {
			w.usage = chunk.Usage
		}
	}
}

func (w *responsesStreamInterceptor) writeSSE(eventType string, data any) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return
	}
	w.ResponseWriter.Write([]byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(dataJSON))))
	w.ResponseWriter.Flush()
}

func writeResponsesError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"error": gin.H{"message": message, "type": "invalid_request_error"},
	})
}

// HandleCodexModels handles GET /v1/codex/models.
// Codex CLI queries this to discover available models.
func HandleCodexModels(c *gin.Context) {
	var names []string
	if enabledModelsFn != nil {
		names = enabledModelsFn()
	}
	result := make([]map[string]any, 0, len(names))
	for _, name := range names {
		result = append(result, map[string]any{
			"id":       name,
			"object":   "model",
			"created":  time.Now().Unix(),
			"owned_by": "custom",
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   result,
	})
}
