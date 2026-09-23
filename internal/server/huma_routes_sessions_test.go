package server_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.kenn.io/agentsview/internal/db"
)

// TestGetSessionIncludesTranscriptFidelity asserts that a session seeded with
// TranscriptFidelity="summary" exposes the field in the GET /api/v1/sessions/{id}
// response. The field is carried by db.Session (embedded in service.SessionDetail)
// with json tag "transcript_fidelity,omitempty", so no additional plumbing is
// needed — this test acts as a regression guard.
func TestGetSessionIncludesTranscriptFidelity(t *testing.T) {
	te := setup(t)
	te.seedSession(t, "agy-fidelity-test", "test-project", 2, func(s *db.Session) {
		s.TranscriptFidelity = "summary"
	})

	w := te.get(t, "/api/v1/sessions/agy-fidelity-test")
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), `"transcript_fidelity":"summary"`)
}

// TestGetSessionPrompts asserts that GET /api/v1/sessions/{id}/prompts
// groups a seeded session's messages into its Prompt/sub-session tree.
func TestGetSessionPrompts(t *testing.T) {
	te := setup(t)
	te.seedSession(t, "prompts-root", "test-project", 2)
	te.seedMessages(t, "prompts-root", 2, func(i int, m *db.Message) {
		if i == 1 {
			m.ToolCalls = []db.ToolCall{{
				ToolName:          "Task",
				ToolUseID:         "tu1",
				SubagentSessionID: "prompts-sub",
				InputJSON:         `{"subagent_type":"Explore"}`,
			}}
		}
	})
	te.seedSession(t, "prompts-sub", "test-project", 1, func(s *db.Session) {
		s.ParentSessionID = new("prompts-root")
		s.RelationshipType = "subagent"
	})

	w := te.get(t, "/api/v1/sessions/prompts-root/prompts")
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var prompts []db.PromptNode
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &prompts), "body: %s", w.Body.String())
	require.Len(t, prompts, 1, "prompts")
	require.Len(t, prompts[0].SubSessions, 1, "prompts[0].SubSessions")
	sub := prompts[0].SubSessions[0]
	assert.Equal(t, "prompts-sub", sub.SessionID, "sub.SessionID")
	assert.Equal(t, "Explore", sub.Role, "sub.Role")
	assert.Equal(t, "subagent", sub.RelationshipType, "sub.RelationshipType")
}

// TestGetSessionExplorerRoots asserts that GET /api/v1/sessions/explorer/roots
// returns only root-level sessions, decorated with their own+descendant
// rolled-up usage, and pages via the same cursor as GET /api/v1/sessions.
func TestGetSessionExplorerRoots(t *testing.T) {
	te := setup(t)
	te.seedSession(t, "explorer-root", "test-project", 1)
	te.seedSession(t, "explorer-sub", "test-project", 1, func(s *db.Session) {
		s.ParentSessionID = new("explorer-root")
		s.RelationshipType = "subagent"
	})

	w := te.get(t, "/api/v1/sessions/explorer/roots")
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	// Assert against the raw JSON, not just the decoded Go struct: a prior
	// version of SessionExplorerRow serialized correctly when inspected as
	// a Go value but silently dropped "usage" from the actual wire format
	// (Session's own MarshalJSON got promoted to SessionExplorerRow via the
	// anonymous embed), which decoding into db.SessionExplorerPage alone
	// would not have caught.
	assert.Contains(t, w.Body.String(), `"usage":`, "body: %s", w.Body.String())

	var page db.SessionExplorerPage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page), "body: %s", w.Body.String())
	require.Len(t, page.Sessions, 1, "page.Sessions")
	assert.Equal(t, "explorer-root", page.Sessions[0].ID, "page.Sessions[0].ID")
}

// TestGetSessionExplorerRootsInvalidCursor asserts a malformed cursor is
// rejected as a client error, matching GET /api/v1/sessions.
func TestGetSessionExplorerRootsInvalidCursor(t *testing.T) {
	te := setup(t)

	w := te.get(t, "/api/v1/sessions/explorer/roots?cursor=not-a-valid-cursor")
	assert.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
}

// TestGetSessionPromptsUnknownSessionReturnsEmptyArray asserts an unknown
// session ID returns 200 with an empty array, matching
// GET /api/v1/sessions/{id}/children's not-found behavior.
func TestGetSessionPromptsUnknownSessionReturnsEmptyArray(t *testing.T) {
	te := setup(t)

	w := te.get(t, "/api/v1/sessions/does-not-exist/prompts")
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.JSONEq(t, "[]", w.Body.String())
}
