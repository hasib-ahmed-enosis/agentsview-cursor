package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.kenn.io/agentsview/internal/export"
	"go.kenn.io/agentsview/internal/money"
)

// PromptNode is one turn within a session: a user ask and everything the
// agent did in direct response to it, plus any sub-agent sessions it
// dispatched. Turns are not a persisted schema concept — they are derived
// at query time from a session's ordered messages (see
// GroupMessagesIntoPrompts).
type PromptNode struct {
	// ID uniquely keys this Prompt for tree/expand state. SessionID is
	// display metadata only: several Prompts can share one underlying
	// session row before a continuation.
	ID           string `json:"id"`
	SessionID    string `json:"sessionId"`
	Ordinal      int    `json:"ordinal"`
	StartedAt    string `json:"startedAt"`
	MessageCount int    `json:"messageCount"`
	// Text is a truncated excerpt of the real user turn's own content —
	// the actual ask, so a Prompt row can show what was asked instead of
	// only "Turn N". Same 180-char truncation convention as the search
	// excerpt builder (truncateExcerpt). Empty for a Prompt whose
	// triggering message has no text content (e.g. an image-only turn).
	Text string `json:"text,omitempty"`
	// IsContinuationStart is true only for the first Prompt of a session
	// that continues an earlier one (relationship_type "continuation"):
	// the merged Session/Tab concept keeps a continuation as more Prompts
	// in the same list rather than a separate root, tagging only the
	// hand-off point rather than every Prompt in the continuation.
	IsContinuationStart bool `json:"isContinuationStart,omitempty"`
	// Usage is this Prompt's own token/cost/duration plus every dispatched
	// sub-session's already-rolled-up totals (see PromptUsage).
	Usage       PromptUsage     `json:"usage"`
	SubSessions []SubSessionRef `json:"subSessions,omitempty"`

	// messages backs Usage's own-activity computation; not part of the
	// public API and cleared before a PromptNode is returned to callers.
	messages []Message
}

// SubSessionRef is a sub-agent session dispatched by a specific Prompt.
// Metadata beyond SessionID is resolved best-effort from the sessions
// table; Role comes from the dispatching Task tool call's own input.
type SubSessionRef struct {
	SessionID        string `json:"sessionId"`
	Role             string `json:"role,omitempty"`
	DisplayName      string `json:"displayName,omitempty"`
	Agent            string `json:"agent,omitempty"`
	StartedAt        string `json:"startedAt,omitempty"`
	RelationshipType string `json:"relationshipType,omitempty"`
	// Usage is this sub-session's own tree rolled up: the sum of its own
	// Prompts' already-rolled-up totals (see PromptUsage). Computed even
	// though the sub-session's own Prompts aren't attached here — the
	// explorer's tree structure still expands lazily, but its numbers are
	// always correct without expanding first.
	Usage PromptUsage `json:"usage"`
}

// PromptUsage is a token/cost/duration rollup for one tree node: its own
// activity plus the sum of every descendant's already-rolled-up totals —
// the flame-graph rule the explorer's data model requires so a node's
// numbers always "add up" once its subtree is (or would be) expanded.
type PromptUsage struct {
	InputTokens         int         `json:"inputTokens"`
	OutputTokens        int         `json:"outputTokens"`
	CacheCreationTokens int         `json:"cacheCreationTokens"`
	CacheReadTokens     int         `json:"cacheReadTokens"`
	TotalTokens         int         `json:"totalTokens"`
	Cost                money.Money `json:"cost"`
	DurationSeconds     float64     `json:"durationSeconds"`
	// HasUsageData is false when none of this node's own messages (nor any
	// descendant's) ever reported a non-zero token counter. Some providers
	// (e.g. Cursor) never record per-message usage at all, so their totals
	// are unknown rather than genuinely zero; the frontend renders that
	// distinction as "x" instead of "0". A node whose messages all report
	// real, legitimately all-zero usage is indistinguishable from this and
	// also renders as "x" — an acceptable trade-off since that case is rare
	// (a real response essentially always reports positive output tokens).
	HasUsageData bool `json:"hasUsageData"`
}

// addUsageTotals sums two PromptUsage. Cost addition can only fail on
// int64 microdollar overflow (money.ErrOverflow), astronomically unlikely
// for any real session but surfaced rather than silently truncated.
func addUsageTotals(a, b PromptUsage) (PromptUsage, error) {
	cost, err := money.Add(a.Cost, b.Cost)
	if err != nil {
		return PromptUsage{}, fmt.Errorf("adding usage totals: %w", err)
	}
	return PromptUsage{
		InputTokens:         a.InputTokens + b.InputTokens,
		OutputTokens:        a.OutputTokens + b.OutputTokens,
		CacheCreationTokens: a.CacheCreationTokens + b.CacheCreationTokens,
		CacheReadTokens:     a.CacheReadTokens + b.CacheReadTokens,
		TotalTokens:         a.TotalTokens + b.TotalTokens,
		Cost:                cost,
		DurationSeconds:     a.DurationSeconds + b.DurationSeconds,
		HasUsageData:        a.HasUsageData || b.HasUsageData,
	}, nil
}

// messageUsageTotals sums the token/cost usage reported on messages' own
// TokenUsage blobs (clamped and priced the same way the existing
// SQL-aggregated usage rollups are), plus a duration spanning their
// earliest to latest parseable Timestamp. A message with no usable token
// counters (no TokenUsage, or one that doesn't parse) contributes zero
// cost/tokens but still counts toward the duration span.
func messageUsageTotals(messages []Message, resolver *export.PricingResolver) (PromptUsage, error) {
	var totals PromptUsage
	for _, m := range messages {
		tokenJSON := string(m.TokenUsage)
		inputTok, outputTok, cacheCrTok, cacheRdTok := clampedUsageTokenCounters(tokenJSON)
		if inputTok == 0 && outputTok == 0 && cacheCrTok == 0 && cacheRdTok == 0 {
			continue
		}
		totals.HasUsageData = true
		cache1hTok := clampedCacheCreation1hTokens(tokenJSON)
		_, lookup := resolver.ResolveAt(
			m.Model, usageLookupModel(m.Model, m.Timestamp), usagePricingTimestamp(m.Timestamp),
		)
		cost, err := lookup.Rates.CostForTokens(inputTok, outputTok, 0, cacheCrTok, cache1hTok, cacheRdTok)
		if err != nil {
			return PromptUsage{}, fmt.Errorf(
				"pricing message %d in session %s: %w", m.Ordinal, m.SessionID, err,
			)
		}
		totals, err = addUsageTotals(totals, PromptUsage{
			InputTokens:         inputTok,
			OutputTokens:        outputTok,
			CacheCreationTokens: cacheCrTok,
			CacheReadTokens:     cacheRdTok,
			TotalTokens:         inputTok + outputTok + cacheCrTok + cacheRdTok,
			Cost:                cost,
		})
		if err != nil {
			return PromptUsage{}, err
		}
	}
	totals.DurationSeconds = messageSpanSeconds(messages)
	return totals, nil
}

// messageSpanSeconds returns the duration from the earliest to the latest
// parseable RFC3339 Timestamp among messages, or 0 if fewer than two
// parse (a single-message span, or no parseable timestamps at all).
func messageSpanSeconds(messages []Message) float64 {
	var first, last time.Time
	haveFirst := false
	for _, m := range messages {
		ts, err := time.Parse(time.RFC3339Nano, m.Timestamp)
		if err != nil {
			continue
		}
		if !haveFirst || ts.Before(first) {
			first = ts
			haveFirst = true
		}
		if ts.After(last) {
			last = ts
		}
	}
	if !haveFirst || !last.After(first) {
		return 0
	}
	return last.Sub(first).Seconds()
}

// GroupMessagesIntoPrompts groups a session's messages, ordered ascending
// by Ordinal with ToolCalls already attached (as GetMessages/
// GetAllMessages return them), into Prompts. Every real user turn starts
// a new Prompt; everything up to (not including) the next real user turn
// belongs to it. A "real" user turn excludes tool-result rows
// (SourceSubtype == "tool_result"), which are the agent's own tool
// output relayed back as a user-role message, not a new ask.
//
// Each sub-agent dispatch found on a message's ToolCalls (a non-empty
// SubagentSessionID) is attributed to the Prompt containing that
// message — the turn that actually dispatched it, not merely the
// session it belongs to.
//
// An empty input returns a non-nil empty slice; in practice a stored
// session always has at least one message, so this path is exercised
// only by a session that has not finished being written.
func GroupMessagesIntoPrompts(messages []Message) []PromptNode {
	prompts := make([]PromptNode, 0)
	var current *PromptNode
	for _, m := range messages {
		if current == nil || isRealUserTurn(m) {
			prompts = append(prompts, PromptNode{
				ID:        fmt.Sprintf("%s:%d", m.SessionID, m.Ordinal),
				SessionID: m.SessionID,
				Ordinal:   m.Ordinal,
				StartedAt: m.Timestamp,
				Text:      truncateExcerpt(m.Content, 180),
			})
			current = &prompts[len(prompts)-1]
		}
		current.MessageCount++
		current.messages = append(current.messages, m)
		for _, tc := range m.ToolCalls {
			if tc.SubagentSessionID == "" {
				continue
			}
			current.SubSessions = append(current.SubSessions, SubSessionRef{
				SessionID: tc.SubagentSessionID,
				Role:      subagentRoleFromInputJSON(tc.InputJSON),
			})
		}
	}
	return prompts
}

func isRealUserTurn(m Message) bool {
	return m.Role == "user" && m.SourceSubtype != "tool_result"
}

// attachUnstampedSubagentSessions appends a SubSessionRef for any child
// session linked via parent_session_id/relationship_type "subagent" that
// GroupMessagesIntoPrompts didn't already attribute to a Prompt through an
// explicit dispatching tool call (a non-empty ToolCall.SubagentSessionID).
// Some providers (e.g. Cursor) link a subagent session to its parent but
// never stamp the dispatching tool call itself, so without this fallback
// their subagents would never appear in the explorer tree at all. Each such
// child is attributed to the last Prompt whose StartedAt does not come
// after the child's own StartedAt (falling back to the first Prompt), since
// there is no dispatching tool call to pin it to an exact turn.
func attachUnstampedSubagentSessions(prompts []PromptNode, children []Session) {
	if len(prompts) == 0 || len(children) == 0 {
		return
	}
	stamped := make(map[string]bool)
	for _, p := range prompts {
		for _, ref := range p.SubSessions {
			stamped[ref.SessionID] = true
		}
	}
	promptTimes := make([]time.Time, len(prompts))
	for i, p := range prompts {
		if ts, err := time.Parse(time.RFC3339Nano, p.StartedAt); err == nil {
			promptTimes[i] = ts
		}
	}
	for _, child := range children {
		if child.RelationshipType != "subagent" || stamped[child.ID] {
			continue
		}
		target := 0
		if child.StartedAt != nil {
			if childTime, err := time.Parse(time.RFC3339Nano, *child.StartedAt); err == nil {
				for i, pt := range promptTimes {
					if !pt.IsZero() && !pt.After(childTime) {
						target = i
					}
				}
			}
		}
		prompts[target].SubSessions = append(
			prompts[target].SubSessions, SubSessionRef{SessionID: child.ID},
		)
	}
}

// taskToolInput is the shape of a Task tool call's input JSON relevant to
// sub-agent attribution, mirroring the fields formatTask (internal/parser/
// content.go) already reads for the rendered "[Task: ...]" label.
type taskToolInput struct {
	SubagentType string `json:"subagent_type"`
	Agent        string `json:"agent"`
}

// subagentRoleFromInputJSON extracts a Task tool call's subagent_type
// (falling back to agent) from its raw input JSON. Best-effort: malformed
// or absent input yields "".
func subagentRoleFromInputJSON(raw string) string {
	if raw == "" {
		return ""
	}
	var in taskToolInput
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		return ""
	}
	if in.SubagentType != "" {
		return in.SubagentType
	}
	return in.Agent
}

// GetSessionPromptsFor returns the Prompt/sub-session-dispatch tree for
// one session: every Prompt (turn) in the session, each carrying the
// sub-agent sessions it dispatched with their cheap display metadata
// resolved, and every Prompt/sub-session carrying its rolled-up
// PromptUsage (own activity plus every descendant's, recursively).
//
// The returned tree's structure is still one level deep — a SubSessionRef
// does not carry its own nested Prompts, so callers still expand a
// sub-session's row list by calling this again with its ID, matching the
// explorer's lazy per-level expansion. Only the *numbers* are computed
// eagerly across the whole subtree, so a row's totals are correct before
// its children are ever fetched for display.
//
// An unknown session ID returns an empty, non-nil slice rather than an
// error, matching GetChildSessions' behavior for a session with no rows.
//
// Implemented purely in terms of the Store interface (GetAllMessages,
// GetSession, LoadPricingResolver) so every backend (SQLite, DuckDB,
// PostgreSQL, ClickHouse) gets this for free via a one-line
// Store.GetSessionPrompts wrapper, without a bespoke per-backend query.
func GetSessionPromptsFor(
	ctx context.Context, store Store, sessionID string,
) ([]PromptNode, error) {
	resolver, err := store.LoadPricingResolver(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading pricing resolver: %w", err)
	}
	return sessionPromptsWithUsage(ctx, store, resolver, sessionID)
}

// sessionPromptsWithUsage is GetSessionPromptsFor's recursive worker: it
// reuses one already-loaded resolver across the whole subtree instead of
// reloading pricing per session.
func sessionPromptsWithUsage(
	ctx context.Context, store Store, resolver *export.PricingResolver, sessionID string,
) ([]PromptNode, error) {
	return sessionPromptsWithUsageVisited(ctx, store, resolver, sessionID, map[string]bool{})
}

// sessionPromptsWithUsageVisited does the actual work, threading a shared
// visited set across both sub-agent dispatch and continuation-chain
// recursion so malformed, cyclic parent/continuation data (A continues B,
// B continues A) can't recurse forever — a session already on the current
// path is simply skipped rather than walked again.
func sessionPromptsWithUsageVisited(
	ctx context.Context, store Store, resolver *export.PricingResolver,
	sessionID string, visited map[string]bool,
) ([]PromptNode, error) {
	if visited[sessionID] {
		return nil, nil
	}
	visited[sessionID] = true

	messages, err := store.GetAllMessages(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("getting messages for %s: %w", sessionID, err)
	}
	prompts := GroupMessagesIntoPrompts(messages)

	children, err := store.GetChildSessions(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("loading child sessions for %s: %w", sessionID, err)
	}
	attachUnstampedSubagentSessions(prompts, children)

	for i := range prompts {
		ownUsage, err := messageUsageTotals(prompts[i].messages, resolver)
		if err != nil {
			return nil, err
		}
		prompts[i].messages = nil
		prompts[i].Usage = ownUsage

		for j := range prompts[i].SubSessions {
			ref := &prompts[i].SubSessions[j]
			sess, err := store.GetSession(ctx, ref.SessionID)
			if err != nil {
				return nil, fmt.Errorf("loading sub-session %s: %w", ref.SessionID, err)
			}
			if sess == nil {
				continue
			}
			ref.DisplayName = subSessionDisplayName(sess)
			ref.Agent = sess.Agent
			if sess.StartedAt != nil {
				ref.StartedAt = *sess.StartedAt
			}
			ref.RelationshipType = sess.RelationshipType

			subPrompts, err := sessionPromptsWithUsageVisited(ctx, store, resolver, ref.SessionID, visited)
			if err != nil {
				return nil, fmt.Errorf("rolling up sub-session %s: %w", ref.SessionID, err)
			}
			subUsage, err := rollupPromptUsage(subPrompts)
			if err != nil {
				return nil, err
			}
			ref.Usage = subUsage
			prompts[i].Usage, err = addUsageTotals(prompts[i].Usage, subUsage)
			if err != nil {
				return nil, err
			}
		}
	}

	continuationPrompts, err := continuationChainPrompts(ctx, store, resolver, sessionID, visited)
	if err != nil {
		return nil, err
	}
	prompts = append(prompts, continuationPrompts...)

	return prompts, nil
}

// continuationChainPrompts stitches a resumed session's own Prompts into
// its origin's Prompt list: a continuation stays part of the same merged
// Session/Tab (see the plan's data model), so its turns are appended here
// rather than surfaced as a separate root, with only the first appended
// Prompt tagged IsContinuationStart to mark where the new underlying DB
// session row began. A session may have more than one child row sharing
// relationship_type "continuation" only through corrupt/duplicated data; the
// earliest by StartedAt (ID as a deterministic tiebreaker for an equal or
// unset StartedAt, since GetChildSessions' own started_at ordering alone
// isn't guaranteed stable across ties) is treated as the real continuation
// and any other is ignored rather than silently merged too.
func continuationChainPrompts(
	ctx context.Context, store Store, resolver *export.PricingResolver,
	sessionID string, visited map[string]bool,
) ([]PromptNode, error) {
	children, err := store.GetChildSessions(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("loading child sessions for %s: %w", sessionID, err)
	}
	continuation, found := earliestContinuation(children)
	if !found {
		return nil, nil
	}
	contPrompts, err := sessionPromptsWithUsageVisited(ctx, store, resolver, continuation.ID, visited)
	if err != nil {
		return nil, fmt.Errorf("stitching continuation %s: %w", continuation.ID, err)
	}
	if len(contPrompts) > 0 {
		contPrompts[0].IsContinuationStart = true
	}
	return contPrompts, nil
}

// earliestContinuation picks the deterministic "real" continuation among a
// session's children: the one with relationship_type "continuation" whose
// (StartedAt, ID) pair sorts first. An unset StartedAt sorts last, so a
// child with real timing data is always preferred over one without.
func earliestContinuation(children []Session) (Session, bool) {
	var best Session
	found := false
	for _, child := range children {
		if child.RelationshipType != "continuation" {
			continue
		}
		if !found || continuationSortsBefore(child, best) {
			best = child
			found = true
		}
	}
	return best, found
}

func continuationSortsBefore(a, b Session) bool {
	aStarted, bStarted := a.StartedAt != nil, b.StartedAt != nil
	if aStarted != bStarted {
		return aStarted
	}
	if aStarted && *a.StartedAt != *b.StartedAt {
		return *a.StartedAt < *b.StartedAt
	}
	return a.ID < b.ID
}

// rollupPromptUsage sums a session's own Prompts' (already rolled up)
// PromptUsage into that session's own subtree total.
func rollupPromptUsage(prompts []PromptNode) (PromptUsage, error) {
	var total PromptUsage
	var err error
	for _, p := range prompts {
		total, err = addUsageTotals(total, p.Usage)
		if err != nil {
			return PromptUsage{}, err
		}
	}
	return total, nil
}

// GetSessionPrompts implements Store.GetSessionPrompts for the SQLite
// backend by delegating to the generic, backend-agnostic implementation.
func (db *DB) GetSessionPrompts(
	ctx context.Context, sessionID string,
) ([]PromptNode, error) {
	return GetSessionPromptsFor(ctx, db, sessionID)
}

// SessionExplorerRow is one root-level row in the paginated session
// explorer list: a session's own metadata plus the same own+descendant
// PromptUsage rollup GetSessionPromptsFor computes for a single session's
// tree, so a row's cost/tokens are correct before it is ever expanded.
type SessionExplorerRow struct {
	Session
	Usage PromptUsage `json:"usage"`
}

// MarshalJSON is required because Session defines its own MarshalJSON
// (value receiver): Go promotes that method to SessionExplorerRow through
// the anonymous embed, which would otherwise make SessionExplorerRow
// silently satisfy json.Marshaler via Session's method alone — serializing
// only the embedded Session's fields and dropping Usage entirely, with no
// compile-time signal that anything is wrong. Decoding Session's own JSON
// back into a generic map and splicing in "usage" reuses Session's
// MarshalJSON as the source of truth for its own fields (quality_signals
// grouping, etc.) instead of duplicating that logic here.
func (r SessionExplorerRow) MarshalJSON() ([]byte, error) {
	sessionJSON, err := r.Session.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshaling session: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(sessionJSON, &fields); err != nil {
		return nil, fmt.Errorf("decoding session JSON: %w", err)
	}
	usageJSON, err := json.Marshal(r.Usage)
	if err != nil {
		return nil, fmt.Errorf("marshaling usage: %w", err)
	}
	fields["usage"] = usageJSON
	return json.Marshal(fields)
}

// UnmarshalJSON is the decode-side counterpart to MarshalJSON, needed for
// the same reason: Session's own UnmarshalJSON (pointer receiver) would
// otherwise be promoted to *SessionExplorerRow and silently decode only the
// embedded Session, leaving Usage zero-valued regardless of what "usage"
// the JSON actually carries.
func (r *SessionExplorerRow) UnmarshalJSON(data []byte) error {
	if err := r.Session.UnmarshalJSON(data); err != nil {
		return fmt.Errorf("decoding session: %w", err)
	}
	var withUsage struct {
		Usage PromptUsage `json:"usage"`
	}
	if err := json.Unmarshal(data, &withUsage); err != nil {
		return fmt.Errorf("decoding usage: %w", err)
	}
	r.Usage = withUsage.Usage
	return nil
}

// SessionExplorerPage is a page of root-level SessionExplorerRow, using the
// same cursor pagination as ListSessions.
type SessionExplorerPage struct {
	Sessions   []SessionExplorerRow `json:"sessions"`
	NextCursor string               `json:"next_cursor,omitempty"`
	Total      int                  `json:"total"`
}

// GetSessionExplorerRootsFor returns a paginated page of root-level
// sessions (IncludeChildren is forced false: the explorer expands a
// session's Prompts/sub-sessions lazily via GetSessionPromptsFor on
// expand, rather than eagerly nesting the whole tree in this list), each
// decorated with its own subtree's rolled-up token/cost totals.
//
// Implemented purely in terms of the Store interface (ListSessions,
// LoadPricingResolver, plus GetSessionPromptsFor's own dependencies), so
// every backend gets this for free via a one-line
// Store.GetSessionExplorerRoots wrapper.
func GetSessionExplorerRootsFor(
	ctx context.Context, store Store, filter SessionFilter,
) (SessionExplorerPage, error) {
	filter.IncludeChildren = false
	// A fork is its own separate root Session (never merged into the
	// session it forked from), so it must NOT be excluded the way the
	// sidebar's default root listing excludes it. A continuation, by
	// contrast, is merged forward into the session it continues (see
	// continuationChainPrompts) and so must be excluded here, or it would
	// also render as a duplicate, prompt-less root of its own.
	filter.RootExcludeRelationships = []string{"subagent", "continuation"}
	page, err := store.ListSessions(ctx, filter)
	if err != nil {
		return SessionExplorerPage{}, err
	}
	resolver, err := store.LoadPricingResolver(ctx)
	if err != nil {
		return SessionExplorerPage{}, fmt.Errorf("loading pricing resolver: %w", err)
	}
	rows := make([]SessionExplorerRow, len(page.Sessions))
	for i, sess := range page.Sessions {
		prompts, err := sessionPromptsWithUsage(ctx, store, resolver, sess.ID)
		if err != nil {
			return SessionExplorerPage{}, fmt.Errorf(
				"computing usage for session %s: %w", sess.ID, err)
		}
		usage, err := rollupPromptUsage(prompts)
		if err != nil {
			return SessionExplorerPage{}, fmt.Errorf(
				"rolling up usage for session %s: %w", sess.ID, err)
		}
		rows[i] = SessionExplorerRow{Session: sess, Usage: usage}
	}
	return SessionExplorerPage{
		Sessions:   rows,
		NextCursor: page.NextCursor,
		Total:      page.Total,
	}, nil
}

// GetSessionExplorerRoots implements Store.GetSessionExplorerRoots for the
// SQLite backend by delegating to the generic, backend-agnostic
// implementation.
func (db *DB) GetSessionExplorerRoots(
	ctx context.Context, filter SessionFilter,
) (SessionExplorerPage, error) {
	return GetSessionExplorerRootsFor(ctx, db, filter)
}

// subSessionDisplayName picks the same fallback chain the session list
// already uses to label a session that was never explicitly renamed:
// display name, then session name, then its first message, then the
// project, then finally its own ID.
func subSessionDisplayName(s *Session) string {
	if s.DisplayName != nil && *s.DisplayName != "" {
		return *s.DisplayName
	}
	if s.SessionName != nil && *s.SessionName != "" {
		return *s.SessionName
	}
	if s.FirstMessage != nil && *s.FirstMessage != "" {
		return *s.FirstMessage
	}
	if s.Project != "" {
		return s.Project
	}
	return s.ID
}
