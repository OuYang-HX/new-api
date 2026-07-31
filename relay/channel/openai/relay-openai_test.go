package openai

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckStreamDataError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		data       string
		wantStatus int
		wantType   string
		wantErr    bool
	}{
		{
			name:       "error_code 429 rate limit",
			statusCode: http.StatusOK,
			data:       `{"error_code": "InferHub.002002010.429", "error_msg": "deny cause quota exceeded"}`,
			wantStatus: http.StatusTooManyRequests,
			wantType:   "rate_limit_error",
			wantErr:    true,
		},
		{
			name:       "error_code 503 server error",
			statusCode: http.StatusOK,
			data:       `{"error_code": "InferHub.002002010.503", "error_msg": "server error"}`,
			wantStatus: http.StatusServiceUnavailable,
			wantType:   "server_error",
			wantErr:    true,
		},
		{
			name:       "non-2xx status preserved",
			statusCode: http.StatusBadGateway,
			data:       `{"error_code": "InferHub.002002010.429", "error_msg": "deny cause quota exceeded"}`,
			wantStatus: http.StatusBadGateway,
			wantType:   "rate_limit_error",
			wantErr:    true,
		},
		{
			name:       "no error fields",
			statusCode: http.StatusOK,
			data:       `{"id": "chatcmpl-123", "choices": []}`,
			wantErr:    false,
		},
		{
			name:       "invalid json",
			statusCode: http.StatusOK,
			data:       `not json`,
			wantErr:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkStreamDataError(tc.statusCode, tc.data)
			if !tc.wantErr {
				assert.Nil(t, err)
				return
			}
			require.NotNil(t, err)
			assert.Equal(t, tc.wantStatus, err.StatusCode)
			oaiErr := err.ToOpenAIError()
			assert.Equal(t, tc.wantType, oaiErr.Type)
			assert.NotEmpty(t, oaiErr.Message)
		})
	}
}
