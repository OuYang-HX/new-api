package token_config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenCacheDeleteByUser(t *testing.T) {
	tc := &TokenCache{cache: map[string]string{
		"templateA:alice": "token-a",
		"templateB:alice": "token-b",
		"templateA:bob":   "token-c",
		"alice":           "legacy-token", // bare-username legacy key
	}}

	tc.DeleteByUser("alice")

	// Every template-scoped entry for alice must be evicted.
	assert.NotContains(t, tc.cache, "templateA:alice")
	assert.NotContains(t, tc.cache, "templateB:alice")
	// Bare-username keys (legacy format) are also evicted.
	assert.NotContains(t, tc.cache, "alice")
	// Other users are untouched.
	assert.Equal(t, "token-c", tc.cache["templateA:bob"])
}

func TestTokenCacheDeleteByUserNoMatch(t *testing.T) {
	tc := &TokenCache{cache: map[string]string{
		"templateA:alice": "token-a",
	}}

	tc.DeleteByUser("charlie")

	assert.Equal(t, map[string]string{"templateA:alice": "token-a"}, tc.cache)
}
