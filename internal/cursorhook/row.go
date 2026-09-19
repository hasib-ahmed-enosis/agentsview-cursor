package cursorhook

import (
	"strings"
	"time"

	"go.kenn.io/agentsview/internal/db"
)

const hookUsageEventKind = "hook"

// UsageRow is one JSONL record from the Cursor token hook.
type UsageRow struct {
	RecordedAt          string `json:"recorded_at"`
	HookEventName       string `json:"hook_event_name"`
	ConversationID      string `json:"conversation_id"`
	GenerationID        string `json:"generation_id"`
	Model               string `json:"model"`
	ModelID             string `json:"model_id"`
	InputTokens         *int64 `json:"input_tokens"`
	OutputTokens        *int64 `json:"output_tokens"`
	CacheReadTokens     *int64 `json:"cache_read_tokens"`
	CacheWriteTokens    *int64 `json:"cache_write_tokens"`
	ContextTokens       *int64 `json:"context_tokens"`
	ContextWindowSize   *int64 `json:"context_window_size"`
	TranscriptPath      string `json:"transcript_path"`
	AgentTranscriptPath string `json:"agent_transcript_path"`
	WorkspaceRoot       string `json:"workspace_root"`
	CursorVersion       string `json:"cursor_version"`
	Status              string `json:"status"`
	SubagentType        string `json:"subagent_type"`
}

func (row UsageRow) resolvedModel() string {
	if m := strings.TrimSpace(row.Model); m != "" {
		return m
	}
	return strings.TrimSpace(row.ModelID)
}

func (row UsageRow) hasUsage() bool {
	return row.InputTokens != nil ||
		row.OutputTokens != nil ||
		row.CacheReadTokens != nil ||
		row.CacheWriteTokens != nil ||
		row.ContextTokens != nil
}

func (row UsageRow) billableEvent() bool {
	switch strings.TrimSpace(row.HookEventName) {
	case "afterAgentResponse", "stop":
		return row.hasUsage() && row.resolvedModel() != ""
	default:
		return false
	}
}

func (row UsageRow) occurredAt() string {
	s := strings.TrimSpace(row.RecordedAt)
	if s == "" {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format(time.RFC3339Nano)
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC().Format(time.RFC3339Nano)
	}
	return s
}

// CursorUsageEvent maps a hook row into the shared cursor_usage_events shape.
func (row UsageRow) CursorUsageEvent() (db.CursorUsageEvent, bool) {
	if !row.billableEvent() {
		return db.CursorUsageEvent{}, false
	}
	ev := db.CursorUsageEvent{
		OccurredAt:       row.occurredAt(),
		Model:            row.resolvedModel(),
		Kind:             hookUsageEventKind,
		InputTokens:      intOrZero(row.InputTokens),
		OutputTokens:     intOrZero(row.OutputTokens),
		CacheWriteTokens: intOrZero(row.CacheWriteTokens),
		CacheReadTokens:  intOrZero(row.CacheReadTokens),
		IsHeadless:       strings.TrimSpace(row.SubagentType) != "",
		SessionID:        CanonicalSessionID(row.ConversationID),
	}
	ev.DedupKey = hookUsageDedupKey(row, ev)
	return ev, true
}

func hookUsageDedupKey(row UsageRow, ev db.CursorUsageEvent) string {
	// Prefer stable hook identifiers when present so reruns stay idempotent.
	gen := strings.TrimSpace(row.GenerationID)
	if gen != "" {
		return db.CursorUsageEventDedupKey(db.CursorUsageEvent{
			OccurredAt:       ev.OccurredAt,
			Model:            ev.Model,
			Kind:             ev.Kind,
			InputTokens:      ev.InputTokens,
			OutputTokens:     ev.OutputTokens,
			CacheWriteTokens: ev.CacheWriteTokens,
			CacheReadTokens:  ev.CacheReadTokens,
			UserID:           gen,
			UserEmail:        strings.TrimSpace(row.ConversationID),
		})
	}
	return db.CursorUsageEventDedupKey(ev)
}

func intOrZero(v *int64) int {
	if v == nil {
		return 0
	}
	return int(*v)
}

// SessionID returns the AgentsView session id when the hook row names one.
func (row UsageRow) SessionID() string {
	return CanonicalSessionID(row.ConversationID)
}

func isHookSessionID(id string) bool {
	if len(id) < 8 || len(id) > 64 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		case r == '-':
		default:
			return false
		}
	}
	return true
}
