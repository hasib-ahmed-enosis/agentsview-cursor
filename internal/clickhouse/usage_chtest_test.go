//go:build chtest

package clickhouse

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.kenn.io/agentsview/internal/db"
	"go.kenn.io/agentsview/internal/money"
)

func TestClickHouseUsageCostFromFixture(t *testing.T) {
	ctx := context.Background()
	local, target := seedFixture(t)
	syncer := newTestSync(t, local, target, SyncOptions{})
	_, err := syncer.Push(ctx, false, nil)
	require.NoError(t, err)
	require.NoError(t, syncer.syncModelPricing(ctx))

	store, err := NewStore(ctx, target)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	want, err := local.GetSessionUsage(ctx, fixtureAlphaID, true)
	require.NoError(t, err)
	require.NotNil(t, want)
	got, err := store.GetSessionUsage(ctx, fixtureAlphaID, true)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want.Cost, got.Cost, "GetSessionUsage cost should match SQLite")
	assert.Equal(t, want.HasCost, got.HasCost)

	// Independent pin: 10 input * $3/MTok + 5 output * $15/MTok = $0.000105.
	eventCost, err := money.CostPerMillion([]money.RatedTokens{
		{Tokens: 10, Rate: money.MustParseDollars("3")},
		{Tokens: 5, Rate: money.MustParseDollars("15")},
	})
	require.NoError(t, err)
	assert.Equal(t, money.MustParseDollars("0.000105"), eventCost)
	assert.GreaterOrEqual(t, got.Cost.Microdollars, eventCost.Microdollars,
		"session cost should include the seeded usage-event cost")

	filter := db.UsageFilter{
		From:     "2026-01-10",
		To:       "2026-01-10",
		Timezone: "UTC",
	}
	wantDaily, err := local.GetDailyUsage(ctx, filter)
	require.NoError(t, err)
	gotDaily, err := store.GetDailyUsage(ctx, filter)
	require.NoError(t, err)
	require.NotEmpty(t, gotDaily.Daily)
	assert.Equal(t, "2026-01-10", gotDaily.Daily[0].Date)
	assert.Equal(t, wantDaily.Totals.TotalCost, gotDaily.Totals.TotalCost,
		"GetDailyUsage totals should match SQLite")
	assert.GreaterOrEqual(t, gotDaily.Totals.TotalCost.Microdollars, eventCost.Microdollars,
		"UTC window covering 2026-01-10 should include the usage-event cost")
}

// TestClickHouseTopSessionsByCostParentLinkage verifies a subagent
// session's TopSessionEntry carries ParentSessionID/RelationshipType from
// sessions.parent_session_id/relationship_type. seedFixture already wires
// fixtureChildID as a subagent of fixtureAlphaID (mirroring the
// tool_calls.subagent_session_id linkage most providers use; Cursor's
// directory-based agent-transcripts/<parent>/subagents/<id> linkage lands
// on the same column, so this query needs no per-provider branching).
func TestClickHouseTopSessionsByCostParentLinkage(t *testing.T) {
	ctx := context.Background()
	local, target := seedFixture(t)
	syncer := newTestSync(t, local, target, SyncOptions{})
	_, err := syncer.Push(ctx, false, nil)
	require.NoError(t, err)
	require.NoError(t, syncer.syncModelPricing(ctx))

	store, err := NewStore(ctx, target)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	top, err := store.GetTopSessionsByCost(ctx, db.UsageFilter{
		From:     "2026-01-10",
		To:       "2026-01-10",
		Timezone: "UTC",
	}, 20)
	require.NoError(t, err)

	byID := map[string]db.TopSessionEntry{}
	for _, entry := range top {
		byID[entry.SessionID] = entry
	}

	root, ok := byID[fixtureAlphaID]
	require.True(t, ok, "expected root session in top sessions")
	assert.Nil(t, root.ParentSessionID, "root.ParentSessionID")
	assert.Empty(t, root.RelationshipType, "root.RelationshipType")

	child, ok := byID[fixtureChildID]
	require.True(t, ok, "expected subagent child session in top sessions")
	require.NotNil(t, child.ParentSessionID, "child.ParentSessionID")
	assert.Equal(t, fixtureAlphaID, *child.ParentSessionID, "child.ParentSessionID")
	assert.Equal(t, "subagent", child.RelationshipType, "child.RelationshipType")
}
