package dto

import (
	"testing"

	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAITextResponseGetOpenAIErrorFromErrorCodeAndMsg(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected *types.OpenAIError
	}{
		{
			name:     "standard error field",
			body:     `{"error": {"type": "rate_limit_error", "message": "rate limit"}}`,
			expected: &types.OpenAIError{Type: "rate_limit_error", Message: "rate limit"},
		},
		{
			name: "error_code and error_msg (rate limit 429)",
			body: `{"error_code": "InferHub.002002010.429", "error_msg": "{\"type\":\"TPH\", \"message\":\"deny cause quota exceeded\"}"}`,
			expected: &types.OpenAIError{
				Type:    "rate_limit_error",
				Message: `{"type":"TPH", "message":"deny cause quota exceeded"}`,
				Code:    "InferHub.002002010.429",
			},
		},
		{
			name: "error_code and error_msg (server error 503)",
			body: `{"error_code": "InferHub.002002010.503", "error_msg": "server error"}`,
			expected: &types.OpenAIError{
				Type:    "server_error",
				Message: "server error",
				Code:    "InferHub.002002010.503",
			},
		},
		{
			name:     "error_msg as plain string without error_code",
			body:     `{"error_msg": "something went wrong"}`,
			expected: &types.OpenAIError{Type: "error", Message: "something went wrong", Code: "error"},
		},
		{
			name:     "error field takes precedence",
			body:     `{"error": {"type": "rate_limit_error", "message": "rate limit"}, "error_code": "InferHub.002002010.429", "error_msg": "deny cause quota exceeded"}`,
			expected: &types.OpenAIError{Type: "rate_limit_error", Message: "rate limit"},
		},
		{
			name:     "no error fields",
			body:     `{}`,
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var resp OpenAITextResponse
			require.NoError(t, kitutil.Unmarshal([]byte(tc.body), &resp))
			assert.Equal(t, tc.expected, resp.GetOpenAIError())
		})
	}
}

func TestSimpleResponseGetOpenAIErrorFromErrorCodeAndMsg(t *testing.T) {
	var resp SimpleResponse
	require.NoError(t, kitutil.Unmarshal([]byte(`{"error_code": "InferHub.002002010.429", "error_msg": "deny cause quota exceeded"}`), &resp))

	err := resp.GetOpenAIError()
	require.NotNil(t, err)
	assert.Equal(t, "rate_limit_error", err.Type)
	assert.Equal(t, "deny cause quota exceeded", err.Message)
	assert.Equal(t, "InferHub.002002010.429", err.Code)
}

func TestExtractStatusCodeFromErrorCode(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"InferHub.002002010.429", 429},
		{"429", 429},
		{"Error.503.ServiceUnavailable", 503},
		{"BadRequest.400", 400},
		{"no digits", 0},
		{"999", 0},
		{"", 0},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, extractStatusCodeFromErrorCode(tc.input))
		})
	}
}
