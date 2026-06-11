// Copyright (C) 2023-2026 QuantumNous
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package protocol_adapter provides protocol translation between different API formats.
// It enables Codex CLI and Claude Code CLI to work with standard OpenAI-compatible channels.
//
// Supported conversions:
//   - Codex CLI (/v1/responses) ↔ OpenAI Chat Completions (/v1/chat/completions)
//   - Claude Code CLI (/v1/messages) ↔ OpenAI Chat Completions (/v1/chat/completions)
//
// This package is designed to be decoupled from the main codebase to minimize merge conflicts.
package protocol_adapter
