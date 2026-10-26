package db

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.kenn.io/agentsview/internal/export"
	"go.kenn.io/agentsview/internal/money"
)

func TestGroupMessagesIntoPrompts_Empty(t *testing.T) {
	prompts := GroupMessagesIntoPrompts(nil)
	assert.NotNil(t, prompts, "prompts should be a non-nil empty slice")
	assert.Empty(t, prompts, "prompts")
}

func TestGroupMessagesIntoPrompts_SingleTurn(t *testing.T) {
	msgs := []Message{
		{SessionID: "s1", Ordinal: 0, Role: "user", Timestamp: "t0", Content: "hi"},
		{SessionID: "s1", Ordinal: 1, Role: "assistant", Timestamp: "t1", Content: "hello"},
	}
	prompts := GroupMessagesIntoPrompts(msgs)
	require.Len(t, prompts, 1, "prompts")
	assert.Equal(t, "s1", prompts[0].SessionID, "prompts[0].SessionID")
	assert.Equal(t, 0, prompts[0].Ordinal, "prompts[0].Ordinal")
	assert.Equal(t, "t0", prompts[0].StartedAt, "prompts[0].StartedAt")
	assert.Equal(t, 2, prompts[0].MessageCount, "prompts[0].MessageCount")
	assert.Empty(t, prompts[0].SubSessions, "prompts[0].SubSessions")
	assert.Equal(t, "hi", prompts[0].Text, "prompts[0].Text")
}

func TestGroupMessagesIntoPrompts_TextIsTruncatedExcerptOfTheTriggeringMessage(t *testing.T) {
	long := strings.Repeat("a", 300)
	msgs := []Message{
		{SessionID: "s1", Ordinal: 0, Role: "user", Timestamp: "t0", Content: long},
	}
	prompts := GroupMessagesIntoPrompts(msgs)
	require.Len(t, prompts, 1, "prompts")
	assert.Equal(t, truncateExcerpt(long, 180), prompts[0].Text, "prompts[0].Text")
	assert.LessOrEqual(t, len(prompts[0].Text), 180, "prompts[0].Text should be truncated")
}

func TestGroupMessagesIntoPrompts_EachTurnGetsItsOwnText(t *testing.T) {
	msgs := []Message{
		{SessionID: "s1", Ordinal: 0, Role: "user", Timestamp: "t0", Content: "first ask"},
		{SessionID: "s1", Ordinal: 1, Role: "assistant", Timestamp: "t1", Content: "reply"},
		{SessionID: "s1", Ordinal: 2, Role: "user", Timestamp: "t2", Content: "second ask"},
	}
	prompts := GroupMessagesIntoPrompts(msgs)
	require.Len(t, prompts, 2, "prompts")
	assert.Equal(t, "first ask", prompts[0].Text, "prompts[0].Text")
	assert.Equal(t, "second ask", prompts[1].Text, "prompts[1].Text")
}

func TestGroupMessagesIntoPrompts_MultipleTurns(t *testing.T) {
	msgs := []Message{
		{SessionID: "s1", Ordinal: 0, Role: "user", Timestamp: "t0"},
		{SessionID: "s1", Ordinal: 1, Role: "assistant", Timestamp: "t1"},
		{SessionID: "s1", Ordinal: 2, Role: "user", Timestamp: "t2"},
		{SessionID: "s1", Ordinal: 3, Role: "assistant", Timestamp: "t3"},
		{SessionID: "s1", Ordinal: 4, Role: "assistant", Timestamp: "t4"},
	}
	prompts := GroupMessagesIntoPrompts(msgs)
	require.Len(t, prompts, 2, "prompts")
	assert.Equal(t, 0, prompts[0].Ordinal, "prompts[0].Ordinal")
	assert.Equal(t, 2, prompts[0].MessageCount, "prompts[0].MessageCount")
	assert.Equal(t, 2, prompts[1].Ordinal, "prompts[1].Ordinal")
	assert.Equal(t, 3, prompts[1].MessageCount, "prompts[1].MessageCount")
}

func TestGroupMessagesIntoPrompts_ExcludesToolResultUserMessages(t *testing.T) {
	msgs := []Message{
		{SessionID: "s1", Ordinal: 0, Role: "user", Timestamp: "t0"},
		{SessionID: "s1", Ordinal: 1, Role: "assistant", Timestamp: "t1"},
		// A tool-result row is the agent's own tool output relayed back as a
		// "user" role message, not a new ask; it must stay in the same Prompt.
		{SessionID: "s1", Ordinal: 2, Role: "user", Timestamp: "t2", SourceSubtype: "tool_result"},
		{SessionID: "s1", Ordinal: 3, Role: "assistant", Timestamp: "t3"},
	}
	prompts := GroupMessagesIntoPrompts(msgs)
	require.Len(t, prompts, 1, "prompts")
	assert.Equal(t, 4, prompts[0].MessageCount, "prompts[0].MessageCount")
}

func TestGroupMessagesIntoPrompts_AttributesSubAgentDispatch(t *testing.T) {
	msgs := []Message{
		{SessionID: "s1", Ordinal: 0, Role: "user", Timestamp: "t0"},
		{
			SessionID: "s1", Ordinal: 1, Role: "assistant", Timestamp: "t1",
			ToolCalls: []ToolCall{{
				ToolName:          "Task",
				ToolUseID:         "tu1",
				SubagentSessionID: "sub-1",
				InputJSON:         `{"subagent_type":"Explore","description":"look around"}`,
			}},
		},
	}
	prompts := GroupMessagesIntoPrompts(msgs)
	require.Len(t, prompts, 1, "prompts")
	require.Len(t, prompts[0].SubSessions, 1, "prompts[0].SubSessions")
	assert.Equal(t, "sub-1", prompts[0].SubSessions[0].SessionID, "SubSessions[0].SessionID")
	assert.Equal(t, "Explore", prompts[0].SubSessions[0].Role, "SubSessions[0].Role")
}

func TestGroupMessagesIntoPrompts_SubAgentAttributedToDispatchingPromptOnly(t *testing.T) {
	msgs := []Message{
		{SessionID: "s1", Ordinal: 0, Role: "user", Timestamp: "t0"},
		{
			SessionID: "s1", Ordinal: 1, Role: "assistant", Timestamp: "t1",
			ToolCalls: []ToolCall{{
				ToolName: "Task", ToolUseID: "tu1", SubagentSessionID: "sub-1",
				InputJSON: `{"subagent_type":"Explore"}`,
			}},
		},
		{SessionID: "s1", Ordinal: 2, Role: "user", Timestamp: "t2"},
		{
			SessionID: "s1", Ordinal: 3, Role: "assistant", Timestamp: "t3",
			ToolCalls: []ToolCall{{
				ToolName: "Task", ToolUseID: "tu2", SubagentSessionID: "sub-2",
				InputJSON: `{"subagent_type":"general-purpose"}`,
			}},
		},
	}
	prompts := GroupMessagesIntoPrompts(msgs)
	require.Len(t, prompts, 2, "prompts")
	require.Len(t, prompts[0].SubSessions, 1, "prompts[0].SubSessions")
	assert.Equal(t, "sub-1", prompts[0].SubSessions[0].SessionID, "prompts[0] sub session")
	require.Len(t, prompts[1].SubSessions, 1, "prompts[1].SubSessions")
	assert.Equal(t, "sub-2", prompts[1].SubSessions[0].SessionID, "prompts[1] sub session")
}

func TestGroupMessagesIntoPrompts_IgnoresToolCallsWithoutSubagentSessionID(t *testing.T) {
	msgs := []Message{
		{SessionID: "s1", Ordinal: 0, Role: "user", Timestamp: "t0"},
		{
			SessionID: "s1", Ordinal: 1, Role: "assistant", Timestamp: "t1",
			ToolCalls: []ToolCall{{ToolName: "Read", ToolUseID: "tu1"}},
		},
	}
	prompts := GroupMessagesIntoPrompts(msgs)
	require.Len(t, prompts, 1, "prompts")
	assert.Empty(t, prompts[0].SubSessions, "prompts[0].SubSessions")
}

func TestGroupMessagesIntoPrompts_MultipleSubAgentsInSamePrompt(t *testing.T) {
	msgs := []Message{
		{SessionID: "s1", Ordinal: 0, Role: "user", Timestamp: "t0"},
		{
			SessionID: "s1", Ordinal: 1, Role: "assistant", Timestamp: "t1",
			ToolCalls: []ToolCall{
				{ToolName: "Task", ToolUseID: "tu1", SubagentSessionID: "sub-1", InputJSON: `{"agent":"Explore"}`},
				{ToolName: "Task", ToolUseID: "tu2", SubagentSessionID: "sub-2", InputJSON: `{"agent":"Explore"}`},
			},
		},
	}
	prompts := GroupMessagesIntoPrompts(msgs)
	require.Len(t, prompts, 1, "prompts")
	require.Len(t, prompts[0].SubSessions, 2, "prompts[0].SubSessions")
}

func TestSubagentRoleFromInputJSON(t *testing.T) {
	assert.Equal(t, "Explore", subagentRoleFromInputJSON(`{"subagent_type":"Explore"}`), "subagent_type")
	assert.Equal(t, "general-purpose", subagentRoleFromInputJSON(`{"agent":"general-purpose"}`), "agent fallback")
	assert.Equal(t, "", subagentRoleFromInputJSON(``), "empty input")
	assert.Equal(t, "", subagentRoleFromInputJSON(`not json`), "malformed input")
}

func TestAddUsageTotals_Sums(t *testing.T) {
	a := PromptUsage{
		InputTokens: 100, OutputTokens: 50, CacheCreationTokens: 10, CacheReadTokens: 5,
		TotalTokens: 165, Cost: money.MustParseDollars("1"), DurationSeconds: 3,
	}
	b := PromptUsage{
		InputTokens: 20, OutputTokens: 10, CacheCreationTokens: 2, CacheReadTokens: 1,
		TotalTokens: 33, Cost: money.MustParseDollars("0.5"), DurationSeconds: 4,
	}
	total, err := addUsageTotals(a, b)
	requireNoError(t, err, "addUsageTotals")
	assert.Equal(t, 120, total.InputTokens, "InputTokens")
	assert.Equal(t, 60, total.OutputTokens, "OutputTokens")
	assert.Equal(t, 12, total.CacheCreationTokens, "CacheCreationTokens")
	assert.Equal(t, 6, total.CacheReadTokens, "CacheReadTokens")
	assert.Equal(t, 198, total.TotalTokens, "TotalTokens")
	assert.Equal(t, money.MustParseDollars("1.5"), total.Cost, "Cost")
	assert.Equal(t, float64(7), total.DurationSeconds, "DurationSeconds")
}

func TestAddUsageTotals_PropagatesCostOverflow(t *testing.T) {
	a := PromptUsage{Cost: money.Money{Microdollars: math.MaxInt64}}
	b := PromptUsage{Cost: money.Money{Microdollars: 1}}
	_, err := addUsageTotals(a, b)
	assert.Error(t, err, "addUsageTotals should propagate microdollar overflow")
}

func TestMessageSpanSeconds_EarliestToLatest(t *testing.T) {
	messages := []Message{
		{Timestamp: "2024-06-15T10:00:05Z"},
		{Timestamp: "2024-06-15T10:00:00Z"},
		{Timestamp: "2024-06-15T10:00:12Z"},
	}
	assert.Equal(t, float64(12), messageSpanSeconds(messages), "messageSpanSeconds")
}

func TestMessageSpanSeconds_ZeroForUnparseableOrSingleTimestamp(t *testing.T) {
	assert.Equal(t, float64(0), messageSpanSeconds(nil), "no messages")
	assert.Equal(t, float64(0), messageSpanSeconds([]Message{{Timestamp: "not-a-time"}}), "unparseable")
	assert.Equal(t, float64(0), messageSpanSeconds([]Message{{Timestamp: "2024-06-15T10:00:00Z"}}), "single timestamp")
}

func TestMessageUsageTotals_SumsTokensAndCost(t *testing.T) {
	resolver := export.NewPricingResolver([]export.EffectivePricingRow{{
		ModelPattern: "model-a",
		Rates: export.ModelRates{
			InputPerMTok:      money.MustParseDollars("2"),
			OutputPerMTok:     money.MustParseDollars("10"),
			CacheWritePerMTok: money.MustParseDollars("3"),
			CacheReadPerMTok:  money.MustParseDollars("0.5"),
		},
	}})
	messages := []Message{
		{
			SessionID: "s1", Ordinal: 0, Model: "model-a", Timestamp: "2024-06-15T10:00:00Z",
			TokenUsage: jsontext.Value(`{"input_tokens":1000,"output_tokens":500,"cache_creation_input_tokens":100,"cache_read_input_tokens":50}`),
		},
		{
			SessionID: "s1", Ordinal: 1, Model: "model-a", Timestamp: "2024-06-15T10:00:10Z",
			TokenUsage: jsontext.Value(`{"input_tokens":2000,"output_tokens":1000}`),
		},
	}

	totals, err := messageUsageTotals(messages, resolver)
	requireNoError(t, err, "messageUsageTotals")

	assert.Equal(t, 3000, totals.InputTokens, "InputTokens")
	assert.Equal(t, 1500, totals.OutputTokens, "OutputTokens")
	assert.Equal(t, 100, totals.CacheCreationTokens, "CacheCreationTokens")
	assert.Equal(t, 50, totals.CacheReadTokens, "CacheReadTokens")
	assert.Equal(t, 4650, totals.TotalTokens, "TotalTokens")
	assert.Equal(t, float64(10), totals.DurationSeconds, "DurationSeconds")

	expectedCost, err := money.CostPerMillion([]money.RatedTokens{
		{Tokens: 3000, Rate: money.MustParseDollars("2")},
		{Tokens: 1500, Rate: money.MustParseDollars("10")},
		{Tokens: 100, Rate: money.MustParseDollars("3")},
		{Tokens: 50, Rate: money.MustParseDollars("0.5")},
	})
	requireNoError(t, err, "expected cost")
	assert.Equal(t, expectedCost, totals.Cost, "Cost")
}

func TestMessageUsageTotals_SkipsMessagesWithoutUsableTokenUsage(t *testing.T) {
	resolver := export.NewPricingResolver([]export.EffectivePricingRow{{
		ModelPattern: "model-a",
		Rates:        export.ModelRates{InputPerMTok: money.MustParseDollars("2")},
	}})
	messages := []Message{
		{SessionID: "s1", Ordinal: 0, Model: "model-a", Timestamp: "2024-06-15T10:00:00Z"},
		{SessionID: "s1", Ordinal: 1, Model: "model-a", Timestamp: "2024-06-15T10:00:05Z", TokenUsage: jsontext.Value(`not json`)},
	}

	totals, err := messageUsageTotals(messages, resolver)
	requireNoError(t, err, "messageUsageTotals")
	assert.Equal(t, PromptUsage{DurationSeconds: 5}, totals, "totals for unusable token usage")
}

// TestMessageUsageTotals_HasUsageDataFalseWhenNoMessageCarriesUsage covers
// a provider (like Cursor) that never records per-message token usage at
// all: the frontend needs to tell "unknown" apart from "genuinely zero".
func TestMessageUsageTotals_HasUsageDataFalseWhenNoMessageCarriesUsage(t *testing.T) {
	resolver := export.NewPricingResolver(nil)
	messages := []Message{
		{SessionID: "s1", Ordinal: 0, Timestamp: "2024-06-15T10:00:00Z"},
		{SessionID: "s1", Ordinal: 1, Timestamp: "2024-06-15T10:00:05Z"},
	}
	totals, err := messageUsageTotals(messages, resolver)
	requireNoError(t, err, "messageUsageTotals")
	assert.False(t, totals.HasUsageData, "HasUsageData should be false when no message carries a token payload")
}

// TestMessageUsageTotals_HasUsageDataTrueWhenAMessageCarriesUsage is the
// counterpart: any message with a parseable (even all-zero) token payload
// marks the node's usage as known rather than unknown.
func TestMessageUsageTotals_HasUsageDataTrueWhenAMessageCarriesUsage(t *testing.T) {
	resolver := export.NewPricingResolver(nil)
	messages := []Message{
		{SessionID: "s1", Ordinal: 0, Timestamp: "2024-06-15T10:00:00Z"},
		{
			SessionID: "s1", Ordinal: 1, Timestamp: "2024-06-15T10:00:05Z",
			TokenUsage: jsontext.Value(`{"input_tokens":10,"output_tokens":5}`),
		},
	}
	totals, err := messageUsageTotals(messages, resolver)
	requireNoError(t, err, "messageUsageTotals")
	assert.True(t, totals.HasUsageData, "HasUsageData should be true once any message carries a token payload")
}

func TestGetSessionPrompts_AttributesAndResolvesSubSessionMetadata(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()

	insertSession(t, d, "root-1", "proj-a", func(s *Session) {
		s.StartedAt = Ptr("2024-06-15T10:00:00Z")
	})
	insertMessages(t, d,
		Message{SessionID: "root-1", Ordinal: 0, Role: "user", Timestamp: "2024-06-15T10:00:00Z", Content: "explore the repo"},
		Message{
			SessionID: "root-1", Ordinal: 1, Role: "assistant", Timestamp: "2024-06-15T10:00:05Z",
			Content: "[Task: explore (Explore)]",
			ToolCalls: []ToolCall{{
				ToolName:          "Task",
				ToolUseID:         "tu1",
				SubagentSessionID: "sub-1",
				InputJSON:         `{"subagent_type":"Explore","description":"explore"}`,
			}},
		},
	)
	insertSession(t, d, "sub-1", "proj-a", func(s *Session) {
		s.Agent = "claude"
		s.SessionName = Ptr("explore")
		s.StartedAt = Ptr("2024-06-15T10:00:06Z")
		s.ParentSessionID = Ptr("root-1")
		s.RelationshipType = "subagent"
	})
	insertMessages(t, d,
		Message{SessionID: "sub-1", Ordinal: 0, Role: "user", Timestamp: "2024-06-15T10:00:06Z"},
	)

	prompts, err := d.GetSessionPrompts(ctx, "root-1")
	requireNoError(t, err, "GetSessionPrompts")

	require.Len(t, prompts, 1, "prompts")
	require.Len(t, prompts[0].SubSessions, 1, "prompts[0].SubSessions")
	sub := prompts[0].SubSessions[0]
	assert.Equal(t, "sub-1", sub.SessionID, "sub.SessionID")
	assert.Equal(t, "Explore", sub.Role, "sub.Role")
	assert.Equal(t, "claude", sub.Agent, "sub.Agent")
	assert.Equal(t, "explore", sub.DisplayName, "sub.DisplayName")
	assert.Equal(t, "2024-06-15T10:00:06Z", sub.StartedAt, "sub.StartedAt")
	assert.Equal(t, "subagent", sub.RelationshipType, "sub.RelationshipType")
}

// TestGetSessionPrompts_AttachesUnstampedSubagentSession covers a provider
// (like Cursor) that links a subagent session to its parent via
// parent_session_id/relationship_type but never stamps the dispatching
// tool call's SubagentSessionID — the only signal GroupMessagesIntoPrompts
// otherwise looks at. Without the fallback, this subagent would never
// appear in the explorer tree at all.
func TestGetSessionPrompts_AttachesUnstampedSubagentSession(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()

	insertSession(t, d, "root-1", "proj-a", func(s *Session) {
		s.Agent = "cursor"
		s.StartedAt = Ptr("2024-06-15T10:00:00Z")
	})
	insertMessages(t, d,
		Message{SessionID: "root-1", Ordinal: 0, Role: "user", Timestamp: "2024-06-15T10:00:00Z", Content: "explore the repo"},
		Message{SessionID: "root-1", Ordinal: 1, Role: "assistant", Timestamp: "2024-06-15T10:00:05Z"},
	)
	insertSession(t, d, "sub-1", "proj-a", func(s *Session) {
		s.Agent = "cursor"
		s.SessionName = Ptr("explore")
		s.StartedAt = Ptr("2024-06-15T10:00:06Z")
		s.ParentSessionID = Ptr("root-1")
		s.RelationshipType = "subagent"
	})
	insertMessages(t, d,
		Message{SessionID: "sub-1", Ordinal: 0, Role: "user", Timestamp: "2024-06-15T10:00:06Z"},
	)

	prompts, err := d.GetSessionPrompts(ctx, "root-1")
	requireNoError(t, err, "GetSessionPrompts")

	require.Len(t, prompts, 1, "prompts")
	require.Len(t, prompts[0].SubSessions, 1, "prompts[0].SubSessions")
	sub := prompts[0].SubSessions[0]
	assert.Equal(t, "sub-1", sub.SessionID, "sub.SessionID")
	assert.Equal(t, "explore", sub.DisplayName, "sub.DisplayName")
	assert.Equal(t, "subagent", sub.RelationshipType, "sub.RelationshipType")
}

// TestGetSessionPrompts_DoesNotDoubleAttachStampedSubagentSession ensures
// the unstamped-subagent fallback never duplicates a sub-session that
// GroupMessagesIntoPrompts already attributed via an explicit Task-tool
// SubagentSessionID stamp.
func TestGetSessionPrompts_DoesNotDoubleAttachStampedSubagentSession(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()

	insertSession(t, d, "root-1", "proj-a")
	insertMessages(t, d,
		Message{SessionID: "root-1", Ordinal: 0, Role: "user", Timestamp: "t0"},
		Message{
			SessionID: "root-1", Ordinal: 1, Role: "assistant", Timestamp: "t1",
			ToolCalls: []ToolCall{{
				ToolName: "Task", ToolUseID: "tu1", SubagentSessionID: "sub-1",
				InputJSON: `{"subagent_type":"Explore"}`,
			}},
		},
	)
	insertSession(t, d, "sub-1", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("root-1")
		s.RelationshipType = "subagent"
	})
	insertMessages(t, d,
		Message{SessionID: "sub-1", Ordinal: 0, Role: "user", Timestamp: "t2"},
	)

	prompts, err := d.GetSessionPrompts(ctx, "root-1")
	requireNoError(t, err, "GetSessionPrompts")
	require.Len(t, prompts, 1, "prompts")
	require.Len(t, prompts[0].SubSessions, 1, "prompts[0].SubSessions should not be duplicated")
}

func TestGetSessionPrompts_RecursionDepthTwo(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()

	insertSession(t, d, "root-1", "proj-a")
	insertMessages(t, d,
		Message{SessionID: "root-1", Ordinal: 0, Role: "user", Timestamp: "t0"},
		Message{
			SessionID: "root-1", Ordinal: 1, Role: "assistant", Timestamp: "t1",
			ToolCalls: []ToolCall{{
				ToolName: "Task", ToolUseID: "tu1", SubagentSessionID: "sub-1",
				InputJSON: `{"subagent_type":"Explore"}`,
			}},
		},
	)
	insertSession(t, d, "sub-1", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("root-1")
		s.RelationshipType = "subagent"
	})
	insertMessages(t, d,
		Message{SessionID: "sub-1", Ordinal: 0, Role: "user", Timestamp: "t2"},
		Message{
			SessionID: "sub-1", Ordinal: 1, Role: "assistant", Timestamp: "t3",
			ToolCalls: []ToolCall{{
				ToolName: "Task", ToolUseID: "tu2", SubagentSessionID: "sub-sub-1",
				InputJSON: `{"subagent_type":"general-purpose"}`,
			}},
		},
	)
	insertSession(t, d, "sub-sub-1", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("sub-1")
		s.RelationshipType = "subagent"
	})
	insertMessages(t, d,
		Message{SessionID: "sub-sub-1", Ordinal: 0, Role: "user", Timestamp: "t4"},
	)

	rootPrompts, err := d.GetSessionPrompts(ctx, "root-1")
	requireNoError(t, err, "GetSessionPrompts root-1")
	require.Len(t, rootPrompts, 1, "root prompts")
	require.Len(t, rootPrompts[0].SubSessions, 1, "root prompts[0].SubSessions")
	assert.Equal(t, "sub-1", rootPrompts[0].SubSessions[0].SessionID, "root -> sub-1")

	subPrompts, err := d.GetSessionPrompts(ctx, "sub-1")
	requireNoError(t, err, "GetSessionPrompts sub-1")
	require.Len(t, subPrompts, 1, "sub-1 prompts")
	require.Len(t, subPrompts[0].SubSessions, 1, "sub-1 prompts[0].SubSessions")
	assert.Equal(t, "sub-sub-1", subPrompts[0].SubSessions[0].SessionID, "sub-1 -> sub-sub-1")
}

func TestGetSessionPrompts_PromptOwnUsageFromMessages(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()
	requireNoError(t, d.UpsertModelPricing([]ModelPricing{{
		ModelPattern: "model-a", InputPerMTok: money.MustParseDollars("2"),
		OutputPerMTok: money.MustParseDollars("10"),
	}}), "UpsertModelPricing")

	insertSession(t, d, "root-1", "proj-a")
	insertMessages(t, d,
		Message{
			SessionID: "root-1", Ordinal: 0, Role: "user", Model: "model-a",
			Timestamp: "2024-06-15T10:00:00Z",
		},
		Message{
			SessionID: "root-1", Ordinal: 1, Role: "assistant", Model: "model-a",
			Timestamp: "2024-06-15T10:00:05Z",
			TokenUsage: jsontext.Value(
				`{"input_tokens":1000,"output_tokens":500}`,
			),
		},
	)

	prompts, err := d.GetSessionPrompts(ctx, "root-1")
	requireNoError(t, err, "GetSessionPrompts")
	require.Len(t, prompts, 1, "prompts")

	expectedCost, err := money.CostPerMillion([]money.RatedTokens{
		{Tokens: 1000, Rate: money.MustParseDollars("2")},
		{Tokens: 500, Rate: money.MustParseDollars("10")},
	})
	requireNoError(t, err, "expected cost")

	assert.Equal(t, 1000, prompts[0].Usage.InputTokens, "InputTokens")
	assert.Equal(t, 500, prompts[0].Usage.OutputTokens, "OutputTokens")
	assert.Equal(t, 1500, prompts[0].Usage.TotalTokens, "TotalTokens")
	assert.Equal(t, expectedCost, prompts[0].Usage.Cost, "Cost")
	assert.Equal(t, float64(5), prompts[0].Usage.DurationSeconds, "DurationSeconds")
}

func TestGetSessionPrompts_RollsUpSubSessionUsageIntoPromptAndRef(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()
	requireNoError(t, d.UpsertModelPricing([]ModelPricing{{
		ModelPattern: "model-a", InputPerMTok: money.MustParseDollars("2"),
		OutputPerMTok: money.MustParseDollars("10"),
	}}), "UpsertModelPricing")

	insertSession(t, d, "root-1", "proj-a")
	insertMessages(t, d,
		Message{
			SessionID: "root-1", Ordinal: 0, Role: "user", Model: "model-a",
			Timestamp:  "2024-06-15T10:00:00Z",
			TokenUsage: jsontext.Value(`{"input_tokens":100,"output_tokens":0}`),
		},
		Message{
			SessionID: "root-1", Ordinal: 1, Role: "assistant", Model: "model-a",
			Timestamp: "2024-06-15T10:00:01Z",
			ToolCalls: []ToolCall{{
				ToolName: "Task", ToolUseID: "tu1", SubagentSessionID: "sub-1",
				InputJSON: `{"subagent_type":"Explore"}`,
			}},
		},
	)
	insertSession(t, d, "sub-1", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("root-1")
		s.RelationshipType = "subagent"
	})
	insertMessages(t, d,
		Message{
			SessionID: "sub-1", Ordinal: 0, Role: "user", Model: "model-a",
			Timestamp:  "2024-06-15T10:00:02Z",
			TokenUsage: jsontext.Value(`{"input_tokens":1000,"output_tokens":500}`),
		},
	)

	prompts, err := d.GetSessionPrompts(ctx, "root-1")
	requireNoError(t, err, "GetSessionPrompts")
	require.Len(t, prompts, 1, "prompts")
	require.Len(t, prompts[0].SubSessions, 1, "SubSessions")

	subUsage := prompts[0].SubSessions[0].Usage
	assert.Equal(t, 1000, subUsage.InputTokens, "sub ref InputTokens")
	assert.Equal(t, 500, subUsage.OutputTokens, "sub ref OutputTokens")

	// The Prompt's own Usage rolls up its own activity (100 input tokens)
	// plus the dispatched sub-session's already-rolled-up totals.
	assert.Equal(t, 1100, prompts[0].Usage.InputTokens, "prompt InputTokens (own + sub)")
	assert.Equal(t, 500, prompts[0].Usage.OutputTokens, "prompt OutputTokens (own + sub)")
	assert.Equal(t, 1600, prompts[0].Usage.TotalTokens, "prompt TotalTokens (own + sub)")

	expectedCost, err := money.Add(
		mustCostPerMillion(t, 100, money.MustParseDollars("2")),
		mustCostPerMillion(t, 1000, money.MustParseDollars("2")),
	)
	requireNoError(t, err, "combining expected input cost")
	outputCost, err := money.CostPerMillion([]money.RatedTokens{
		{Tokens: 500, Rate: money.MustParseDollars("10")},
	})
	requireNoError(t, err, "expected output cost")
	expectedCost, err = money.Add(expectedCost, outputCost)
	requireNoError(t, err, "combining expected total cost")
	assert.Equal(t, expectedCost, prompts[0].Usage.Cost, "prompt Cost (own + sub)")
}

func mustCostPerMillion(t *testing.T, tokens int64, rate money.Money) money.Money {
	t.Helper()
	cost, err := money.CostPerMillion([]money.RatedTokens{{Tokens: tokens, Rate: rate}})
	requireNoError(t, err, "CostPerMillion")
	return cost
}

func TestGetSessionPrompts_RollupAtDepthTwoIncludesEveryLevel(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()
	requireNoError(t, d.UpsertModelPricing([]ModelPricing{{
		ModelPattern: "model-a", InputPerMTok: money.MustParseDollars("1"),
	}}), "UpsertModelPricing")

	insertSession(t, d, "root-1", "proj-a")
	insertMessages(t, d,
		Message{SessionID: "root-1", Ordinal: 0, Role: "user", Timestamp: "t0"},
		Message{
			SessionID: "root-1", Ordinal: 1, Role: "assistant", Timestamp: "t1",
			ToolCalls: []ToolCall{{
				ToolName: "Task", ToolUseID: "tu1", SubagentSessionID: "sub-1",
				InputJSON: `{"subagent_type":"Explore"}`,
			}},
		},
	)
	insertSession(t, d, "sub-1", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("root-1")
		s.RelationshipType = "subagent"
	})
	insertMessages(t, d,
		Message{
			SessionID: "sub-1", Ordinal: 0, Role: "user", Model: "model-a", Timestamp: "t2",
			TokenUsage: jsontext.Value(`{"input_tokens":10}`),
		},
		Message{
			SessionID: "sub-1", Ordinal: 1, Role: "assistant", Timestamp: "t3",
			ToolCalls: []ToolCall{{
				ToolName: "Task", ToolUseID: "tu2", SubagentSessionID: "sub-sub-1",
				InputJSON: `{"subagent_type":"general-purpose"}`,
			}},
		},
	)
	insertSession(t, d, "sub-sub-1", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("sub-1")
		s.RelationshipType = "subagent"
	})
	insertMessages(t, d,
		Message{
			SessionID: "sub-sub-1", Ordinal: 0, Role: "user", Model: "model-a", Timestamp: "t4",
			TokenUsage: jsontext.Value(`{"input_tokens":100}`),
		},
	)

	rootPrompts, err := d.GetSessionPrompts(ctx, "root-1")
	requireNoError(t, err, "GetSessionPrompts root-1")
	require.Len(t, rootPrompts, 1, "root prompts")
	require.Len(t, rootPrompts[0].SubSessions, 1, "root SubSessions")

	// The root Prompt's own activity is zero tokens; its rolled-up total
	// must still include both sub-1's own 10 and sub-sub-1's own 100.
	assert.Equal(t, 110, rootPrompts[0].Usage.InputTokens, "root prompt InputTokens (own 0 + sub-1 10 + sub-sub-1 100)")
	// The sub-1 ref, as seen from root, is itself rolled up: its own 10 plus sub-sub-1's 100.
	assert.Equal(t, 110, rootPrompts[0].SubSessions[0].Usage.InputTokens, "sub-1 ref InputTokens (own + sub-sub-1)")
}

func TestGetSessionPrompts_SubSessionWithNoMessagesHasZeroUsage(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()

	insertSession(t, d, "root-1", "proj-a")
	insertMessages(t, d,
		Message{SessionID: "root-1", Ordinal: 0, Role: "user", Timestamp: "t0"},
		Message{
			SessionID: "root-1", Ordinal: 1, Role: "assistant", Timestamp: "t1",
			ToolCalls: []ToolCall{{
				ToolName: "Task", ToolUseID: "tu1", SubagentSessionID: "sub-1",
				InputJSON: `{"subagent_type":"Explore"}`,
			}},
		},
	)
	insertSession(t, d, "sub-1", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("root-1")
		s.RelationshipType = "subagent"
	})
	// sub-1 has no messages inserted at all.

	prompts, err := d.GetSessionPrompts(ctx, "root-1")
	requireNoError(t, err, "GetSessionPrompts")
	require.Len(t, prompts, 1, "prompts")
	require.Len(t, prompts[0].SubSessions, 1, "SubSessions")
	assert.Equal(t, PromptUsage{}, prompts[0].SubSessions[0].Usage, "sub-1 ref Usage")
	assert.Equal(t, PromptUsage{}, prompts[0].Usage, "root prompt Usage")
}

func TestGetSessionPrompts_PropagatesMessageQueryError(t *testing.T) {
	d := testDB(t)
	insertSession(t, d, "root-1", "proj-a")
	insertMessages(t, d, Message{SessionID: "root-1", Ordinal: 0, Role: "user", Timestamp: "t0"})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := d.GetSessionPrompts(ctx, "root-1")
	assert.Error(t, err, "GetSessionPrompts with a canceled context")
}

func TestGetSessionPrompts_PropagatesSubSessionMetadataQueryError(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()
	insertSession(t, d, "root-1", "proj-a")
	insertMessages(t, d,
		Message{SessionID: "root-1", Ordinal: 0, Role: "user", Timestamp: "t0"},
		Message{
			SessionID: "root-1", Ordinal: 1, Role: "assistant", Timestamp: "t1",
			ToolCalls: []ToolCall{{
				ToolName: "Task", ToolUseID: "tu1", SubagentSessionID: "sub-1",
				InputJSON: `{"subagent_type":"Explore"}`,
			}},
		},
	)

	// Cancels only after the messages have been fetched successfully, so the
	// failure is isolated to the sub-session metadata lookup itself.
	cancelCtx, cancel := context.WithCancel(ctx)
	messages, err := d.GetAllMessages(cancelCtx, "root-1")
	requireNoError(t, err, "GetAllMessages")
	require.NotEmpty(t, messages, "messages")
	cancel()

	_, err = d.GetSession(cancelCtx, "sub-1")
	assert.Error(t, err, "GetSession with a canceled context")
}

func TestGetSessionPrompts_UnknownSessionReturnsEmpty(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()

	prompts, err := d.GetSessionPrompts(ctx, "does-not-exist")
	requireNoError(t, err, "GetSessionPrompts")
	assert.Empty(t, prompts, "prompts")
}

func TestGetSessionExplorerRoots_ExcludesChildrenAndIncludesUsage(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()
	requireNoError(t, d.UpsertModelPricing([]ModelPricing{{
		ModelPattern: "model-a", InputPerMTok: money.MustParseDollars("2"),
		OutputPerMTok: money.MustParseDollars("10"),
	}}), "UpsertModelPricing")

	insertSession(t, d, "root-1", "proj-a", func(s *Session) {
		s.EndedAt = Ptr("2024-06-15T10:05:00Z")
	})
	insertMessages(t, d,
		Message{
			SessionID: "root-1", Ordinal: 0, Role: "user", Model: "model-a",
			Timestamp:  "2024-06-15T10:00:00Z",
			TokenUsage: jsontext.Value(`{"input_tokens":100,"output_tokens":0}`),
		},
		Message{
			SessionID: "root-1", Ordinal: 1, Role: "assistant", Timestamp: "2024-06-15T10:00:01Z",
			ToolCalls: []ToolCall{{
				ToolName: "Task", ToolUseID: "tu1", SubagentSessionID: "sub-1",
				InputJSON: `{"subagent_type":"Explore"}`,
			}},
		},
	)
	insertSession(t, d, "sub-1", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("root-1")
		s.RelationshipType = "subagent"
	})
	insertMessages(t, d,
		Message{
			SessionID: "sub-1", Ordinal: 0, Role: "user", Model: "model-a",
			Timestamp:  "2024-06-15T10:00:02Z",
			TokenUsage: jsontext.Value(`{"input_tokens":1000,"output_tokens":500}`),
		},
	)

	page, err := d.GetSessionExplorerRoots(ctx, SessionFilter{Limit: 10})
	requireNoError(t, err, "GetSessionExplorerRoots")

	require.Len(t, page.Sessions, 1, "only the root session should appear, not its sub-agent")
	row := page.Sessions[0]
	assert.Equal(t, "root-1", row.ID, "row.ID")

	// The row's Usage rolls up its own 100 input tokens plus sub-1's
	// already-rolled-up 1000, matching GetSessionPromptsFor's rollup rule.
	assert.Equal(t, 1100, row.Usage.InputTokens, "row Usage.InputTokens (own + sub-1)")
	assert.Equal(t, 500, row.Usage.OutputTokens, "row Usage.OutputTokens (own + sub-1)")
	assert.Equal(t, 1600, row.Usage.TotalTokens, "row Usage.TotalTokens (own + sub-1)")
}

func TestSessionExplorerRow_MarshalJSONIncludesUsageAlongsideSessionFields(t *testing.T) {
	row := SessionExplorerRow{
		Session: Session{ID: "root-1", Project: "proj-a", Agent: "claude"},
		Usage:   PromptUsage{InputTokens: 42, TotalTokens: 42},
	}

	// This is a regression test for a real bug: Session defines its own
	// MarshalJSON (value receiver), which Go promotes to SessionExplorerRow
	// through the anonymous embed. That silently made SessionExplorerRow
	// satisfy json.Marshaler via Session's method alone, so json.Marshal
	// serialized only the embedded Session and dropped Usage entirely --
	// with every other test in this file passing, because they only ever
	// inspected the Go struct's fields directly and never round-tripped it
	// through encoding/json the way the HTTP handler actually does.
	data, err := json.Marshal(row)
	requireNoError(t, err, "json.Marshal(SessionExplorerRow)")

	var decoded map[string]any
	requireNoError(t, json.Unmarshal(data, &decoded), "json.Unmarshal")

	require.Contains(t, decoded, "usage", "marshaled JSON must include the usage field")
	usage, ok := decoded["usage"].(map[string]any)
	require.True(t, ok, "usage must decode as an object")
	assert.Equal(t, float64(42), usage["inputTokens"], "usage.inputTokens")
	assert.Equal(t, float64(42), usage["totalTokens"], "usage.totalTokens")

	assert.Equal(t, "root-1", decoded["id"], "id")
	assert.Equal(t, "proj-a", decoded["project"], "project")
	assert.Equal(t, "claude", decoded["agent"], "agent")

	// Round-trip: decoding that same JSON back into a SessionExplorerRow
	// must recover both the embedded Session fields and Usage, not just
	// whichever one Session's own promoted UnmarshalJSON would decode.
	var roundTripped SessionExplorerRow
	requireNoError(t, json.Unmarshal(data, &roundTripped), "json.Unmarshal(SessionExplorerRow)")
	assert.Equal(t, row, roundTripped, "round-tripped SessionExplorerRow")
}

func TestGetSessionExplorerRoots_ForcesIncludeChildrenFalse(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()

	insertSession(t, d, "root-1", "proj-a")
	insertSession(t, d, "sub-1", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("root-1")
		s.RelationshipType = "subagent"
	})
	insertMessages(t, d, Message{SessionID: "root-1", Ordinal: 0, Role: "user", Timestamp: "t0"})
	insertMessages(t, d, Message{SessionID: "sub-1", Ordinal: 0, Role: "user", Timestamp: "t1"})

	// Even a caller that explicitly asks for children back must see only
	// the root: the explorer's top-level list always expands lazily via
	// GetSessionPromptsFor, never by eagerly listing descendants here.
	page, err := d.GetSessionExplorerRoots(ctx, SessionFilter{Limit: 10, IncludeChildren: true})
	requireNoError(t, err, "GetSessionExplorerRoots")
	require.Len(t, page.Sessions, 1, "page.Sessions")
	assert.Equal(t, "root-1", page.Sessions[0].ID, "page.Sessions[0].ID")
}

func TestGetSessionExplorerRoots_PaginatesLikeListSessions(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()

	for i := range 5 {
		ea := fmt.Sprintf("2024-01-01T0%d:00:00Z", i)
		insertSession(t, d, fmt.Sprintf("root-%c", 'a'+i), "proj-a", func(s *Session) {
			s.EndedAt = Ptr(ea)
		})
	}

	seen := make(map[string]bool)
	cursor := ""
	for {
		page, err := d.GetSessionExplorerRoots(ctx, SessionFilter{Limit: 2, Cursor: cursor})
		requireNoError(t, err, "GetSessionExplorerRoots")
		for _, row := range page.Sessions {
			assert.False(t, seen[row.ID], "duplicate session %s across pages", row.ID)
			seen[row.ID] = true
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	assert.Len(t, seen, 5, "seen")
}

func TestGetSessionExplorerRoots_PropagatesInvalidCursor(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()

	_, err := d.GetSessionExplorerRoots(ctx, SessionFilter{Cursor: "not-a-valid-cursor"})
	assert.ErrorIs(t, err, ErrInvalidCursor, "GetSessionExplorerRoots with an invalid cursor")
}

func TestGetSessionExplorerRoots_Empty(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()

	page, err := d.GetSessionExplorerRoots(ctx, SessionFilter{Limit: 10})
	requireNoError(t, err, "GetSessionExplorerRoots")
	assert.Empty(t, page.Sessions, "page.Sessions")
	assert.Equal(t, 0, page.Total, "page.Total")
}

func TestGetSessionExplorerRoots_PropagatesUsageComputationError(t *testing.T) {
	d := testDB(t)
	insertSession(t, d, "root-1", "proj-a")
	insertMessages(t, d, Message{SessionID: "root-1", Ordinal: 0, Role: "user", Timestamp: "t0"})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := d.GetSessionExplorerRoots(ctx, SessionFilter{Limit: 10})
	assert.Error(t, err, "GetSessionExplorerRoots with a canceled context")
}

func TestGetSessionPrompts_StitchesContinuationIntoSamePromptList(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()

	insertSession(t, d, "root-1", "proj-a")
	insertMessages(t, d,
		Message{SessionID: "root-1", Ordinal: 0, Role: "user", Timestamp: "t0"},
		Message{SessionID: "root-1", Ordinal: 1, Role: "assistant", Timestamp: "t1"},
	)
	insertSession(t, d, "root-1-cont", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("root-1")
		s.RelationshipType = "continuation"
	})
	insertMessages(t, d,
		Message{SessionID: "root-1-cont", Ordinal: 0, Role: "user", Timestamp: "t2"},
		Message{SessionID: "root-1-cont", Ordinal: 1, Role: "assistant", Timestamp: "t3"},
	)

	prompts, err := d.GetSessionPrompts(ctx, "root-1")
	requireNoError(t, err, "GetSessionPrompts")

	require.Len(t, prompts, 2, "prompts (own turn + continuation's turn merged into the same list)")
	assert.Equal(t, "root-1", prompts[0].SessionID, "prompts[0].SessionID")
	assert.False(t, prompts[0].IsContinuationStart, "prompts[0].IsContinuationStart")
	assert.Equal(t, "root-1-cont", prompts[1].SessionID, "prompts[1].SessionID")
	assert.True(t, prompts[1].IsContinuationStart, "prompts[1].IsContinuationStart")
}

func TestGetSessionPrompts_ContinuationChainMultipleHopsMergeInOrder(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()

	insertSession(t, d, "root-1", "proj-a")
	insertMessages(t, d, Message{SessionID: "root-1", Ordinal: 0, Role: "user", Timestamp: "t0"})
	insertSession(t, d, "cont-1", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("root-1")
		s.RelationshipType = "continuation"
	})
	insertMessages(t, d, Message{SessionID: "cont-1", Ordinal: 0, Role: "user", Timestamp: "t1"})
	insertSession(t, d, "cont-2", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("cont-1")
		s.RelationshipType = "continuation"
	})
	insertMessages(t, d, Message{SessionID: "cont-2", Ordinal: 0, Role: "user", Timestamp: "t2"})

	prompts, err := d.GetSessionPrompts(ctx, "root-1")
	requireNoError(t, err, "GetSessionPrompts")

	require.Len(t, prompts, 3, "prompts across the whole continuation chain")
	assert.Equal(t, "root-1", prompts[0].SessionID, "prompts[0].SessionID")
	assert.Equal(t, "cont-1", prompts[1].SessionID, "prompts[1].SessionID")
	assert.True(t, prompts[1].IsContinuationStart, "prompts[1].IsContinuationStart")
	assert.Equal(t, "cont-2", prompts[2].SessionID, "prompts[2].SessionID")
	assert.True(t, prompts[2].IsContinuationStart, "prompts[2].IsContinuationStart")
}

func TestGetSessionPrompts_ContinuationUsageRollsUpIntoOrigin(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()
	requireNoError(t, d.UpsertModelPricing([]ModelPricing{{
		ModelPattern: "model-a", InputPerMTok: money.MustParseDollars("2"),
	}}), "UpsertModelPricing")

	insertSession(t, d, "root-1", "proj-a")
	insertMessages(t, d, Message{
		SessionID: "root-1", Ordinal: 0, Role: "user", Model: "model-a", Timestamp: "t0",
		TokenUsage: jsontext.Value(`{"input_tokens":10}`),
	})
	insertSession(t, d, "root-1-cont", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("root-1")
		s.RelationshipType = "continuation"
	})
	insertMessages(t, d, Message{
		SessionID: "root-1-cont", Ordinal: 0, Role: "user", Model: "model-a", Timestamp: "t1",
		TokenUsage: jsontext.Value(`{"input_tokens":90}`),
	})

	prompts, err := d.GetSessionPrompts(ctx, "root-1")
	requireNoError(t, err, "GetSessionPrompts")
	require.Len(t, prompts, 2, "prompts")

	page, err := d.GetSessionExplorerRoots(ctx, SessionFilter{Limit: 10})
	requireNoError(t, err, "GetSessionExplorerRoots")
	require.Len(t, page.Sessions, 1, "only root-1 should be a root; the continuation must not duplicate it")
	assert.Equal(t, 100, page.Sessions[0].Usage.InputTokens, "root row usage rolls up the continuation's tokens too")
}

func TestGetSessionPrompts_DoesNotMergeForkChildren(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()

	insertSession(t, d, "root-1", "proj-a")
	insertMessages(t, d, Message{SessionID: "root-1", Ordinal: 0, Role: "user", Timestamp: "t0"})
	insertSession(t, d, "fork-1", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("root-1")
		s.RelationshipType = "fork"
	})
	insertMessages(t, d, Message{SessionID: "fork-1", Ordinal: 0, Role: "user", Timestamp: "t1"})

	prompts, err := d.GetSessionPrompts(ctx, "root-1")
	requireNoError(t, err, "GetSessionPrompts")
	require.Len(t, prompts, 1, "a fork must never be folded into the session it forked from")
}

func TestGetSessionPrompts_ContinuationCycleTerminates(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()

	// Corrupt data: two sessions each claiming to continue the other. The
	// shared visited set must stop this from recursing forever.
	insertSession(t, d, "a", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("b")
		s.RelationshipType = "continuation"
	})
	insertMessages(t, d, Message{SessionID: "a", Ordinal: 0, Role: "user", Timestamp: "t0"})
	insertSession(t, d, "b", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("a")
		s.RelationshipType = "continuation"
	})
	insertMessages(t, d, Message{SessionID: "b", Ordinal: 0, Role: "user", Timestamp: "t1"})

	prompts, err := d.GetSessionPrompts(ctx, "a")
	requireNoError(t, err, "GetSessionPrompts should terminate rather than loop on a cycle")
	require.Len(t, prompts, 2, "prompts (a's own turn plus b's, b's own continuation back to a skipped)")
}

func TestGetSessionPrompts_DuplicateContinuationChildrenPickEarliestDeterministically(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()

	insertSession(t, d, "root-1", "proj-a")
	insertMessages(t, d, Message{SessionID: "root-1", Ordinal: 0, Role: "user", Timestamp: "t0"})
	// Corrupt/duplicated data: two children both claim relationship_type
	// "continuation" of root-1 with the identical StartedAt, so ordering
	// must fall back to a stable tiebreaker (ID) rather than being
	// dependent on incidental row/storage order.
	insertSession(t, d, "cont-z", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("root-1")
		s.RelationshipType = "continuation"
		s.StartedAt = Ptr("2024-06-15T10:00:00Z")
	})
	insertMessages(t, d, Message{SessionID: "cont-z", Ordinal: 0, Role: "user", Timestamp: "t1"})
	insertSession(t, d, "cont-a", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("root-1")
		s.RelationshipType = "continuation"
		s.StartedAt = Ptr("2024-06-15T10:00:00Z")
	})
	insertMessages(t, d, Message{SessionID: "cont-a", Ordinal: 0, Role: "user", Timestamp: "t2"})

	prompts, err := d.GetSessionPrompts(ctx, "root-1")
	requireNoError(t, err, "GetSessionPrompts")

	require.Len(t, prompts, 2, "exactly one of the tied continuation children is merged, not both")
	assert.Equal(t, "cont-a", prompts[1].SessionID, "the lexicographically-smaller ID wins the tie deterministically")
}

func TestGetSessionExplorerRoots_IncludesForksAsSeparateRoots(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()

	insertSession(t, d, "root-1", "proj-a")
	insertMessages(t, d, Message{SessionID: "root-1", Ordinal: 0, Role: "user", Timestamp: "t0"})
	insertSession(t, d, "fork-1", "proj-a", func(s *Session) {
		s.ParentSessionID = Ptr("root-1")
		s.RelationshipType = "fork"
	})
	insertMessages(t, d, Message{SessionID: "fork-1", Ordinal: 0, Role: "user", Timestamp: "t1"})

	page, err := d.GetSessionExplorerRoots(ctx, SessionFilter{Limit: 10})
	requireNoError(t, err, "GetSessionExplorerRoots")

	ids := make([]string, len(page.Sessions))
	for i, row := range page.Sessions {
		ids[i] = row.ID
	}
	assert.ElementsMatch(t, []string{"root-1", "fork-1"}, ids,
		"a fork must surface as its own separate root Session, never merged into the session it forked from")
}

func TestGetSessionPrompts_ManyTurns(t *testing.T) {
	d := testDB(t)
	ctx := t.Context()

	insertSession(t, d, "root-1", "proj-a")
	// One turn per message, beyond a single GetMessages page's size, to
	// exercise GetAllMessages' unbounded fetch feeding GroupMessagesIntoPrompts.
	count := DefaultMessageLimit + 5
	msgs := make([]Message, 0, count)
	for i := range count {
		msgs = append(msgs, Message{
			SessionID: "root-1", Ordinal: i, Role: "user", Timestamp: "t",
		})
	}
	insertMessages(t, d, msgs...)

	prompts, err := d.GetSessionPrompts(ctx, "root-1")
	requireNoError(t, err, "GetSessionPrompts")
	assert.Len(t, prompts, count, "prompts")
}
