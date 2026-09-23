<!-- ABOUTME: Full-page, unbounded, filterable Session -> Prompt -> Sub-session
     explorer. Unlike the Usage page's fixed top-N widget, this paginates
     every root Session (a "Load more" cursor, matching the Activity page's
     SessionsTable precedent) and reuses ConversationTree/ConversationTreeNode
     for the recursive tree rendering. -->
<script lang="ts">
  import { onMount } from "svelte";
  import { sessionExplorer } from "../../stores/sessionExplorer.svelte.js";
  import { sessions } from "../../stores/sessions.svelte.js";
  import { m } from "../../i18n/index.js";
  import ConversationTree from "../usage/ConversationTree.svelte";
  import type { SessionTreeNode } from "../usage/ConversationTreeNode.svelte";

  function displayNameFor(row: (typeof sessionExplorer.rows)[number]): string {
    return row.display_name || row.first_message || row.project || row.id;
  }

  // In-page search: filters the rows already loaded for this page (not a
  // server-side query), so it only ever narrows what "Load more" has
  // fetched so far.
  let searchQuery = $state("");

  function matchesSearch(row: (typeof sessionExplorer.rows)[number], query: string): boolean {
    const haystack = [
      displayNameFor(row),
      row.project,
      row.agent,
      row.id,
      row.first_message ?? "",
    ]
      .join(" ")
      .toLowerCase();
    return haystack.includes(query);
  }

  const filteredRows = $derived(
    searchQuery.trim()
      ? sessionExplorer.rows.filter((row) => matchesSearch(row, searchQuery.trim().toLowerCase()))
      : sessionExplorer.rows,
  );

  const rows = $derived<SessionTreeNode[]>(
    filteredRows.map((row) => ({
      kind: "session" as const,
      id: row.id,
      sessionId: row.id,
      isRoot: true,
      displayName: displayNameFor(row),
      agent: row.agent,
      project: row.project,
      startedAt: row.started_at ?? undefined,
      relationshipType: row.relationship_type,
      cost: row.usage.cost,
      tokens: {
        input: row.usage.inputTokens,
        output: row.usage.outputTokens,
        cacheWrite: row.usage.cacheCreationTokens,
        cacheRead: row.usage.cacheReadTokens,
        total: row.usage.totalTokens,
      },
    })),
  );

  onMount(() => {
    // Seed filters from the Sessions page's current project/agent selection
    // so arriving here (e.g. via "View all sessions") carries over context.
    sessionExplorer.setFilters({
      project: sessions.filters.project,
      agent: sessions.filters.agent,
    });
  });
</script>

<div class="explorer-page">
  <div class="explorer-header">
    <h2 class="explorer-title">{m.session_explorer_title()}</h2>
    <span class="explorer-total">{m.session_explorer_total_sessions({ count: sessionExplorer.total })}</span>
  </div>

  <div class="explorer-filters">
    <label class="filter-field search-field">
      <span>{m.session_explorer_search()}</span>
      <input
        type="search"
        value={searchQuery}
        oninput={(e) => (searchQuery = e.currentTarget.value)}
        placeholder={m.session_explorer_search_placeholder()}
      />
    </label>
    <label class="filter-field">
      <span>{m.session_explorer_filter_project()}</span>
      <input
        type="text"
        value={sessionExplorer.filters.project}
        oninput={(e) => sessionExplorer.setFilters({ project: e.currentTarget.value })}
        placeholder={m.session_explorer_filter_project_placeholder()}
      />
    </label>
    <label class="filter-field">
      <span>{m.session_explorer_filter_agent()}</span>
      <input
        type="text"
        value={sessionExplorer.filters.agent}
        oninput={(e) => sessionExplorer.setFilters({ agent: e.currentTarget.value })}
        placeholder={m.session_explorer_filter_agent_placeholder()}
      />
    </label>
    <label class="filter-field">
      <span>{m.session_explorer_filter_from()}</span>
      <input
        type="date"
        value={sessionExplorer.filters.dateFrom}
        oninput={(e) => sessionExplorer.setFilters({ dateFrom: e.currentTarget.value })}
      />
    </label>
    <label class="filter-field">
      <span>{m.session_explorer_filter_to()}</span>
      <input
        type="date"
        value={sessionExplorer.filters.dateTo}
        oninput={(e) => sessionExplorer.setFilters({ dateTo: e.currentTarget.value })}
      />
    </label>
  </div>

  <ConversationTree
    {rows}
    title=""
    error={sessionExplorer.error}
    onRetry={() => sessionExplorer.reload()}
    emptyMessage={searchQuery.trim() && sessionExplorer.rows.length > 0
      ? m.session_explorer_no_search_results({ query: searchQuery.trim() })
      : m.session_explorer_empty()}
    showRank={false}
  />

  {#if sessionExplorer.hasMore}
    <div class="load-more-row">
      <button
        class="load-more-btn"
        disabled={sessionExplorer.loading}
        onclick={() => sessionExplorer.loadMore()}
      >
        {sessionExplorer.loading ? m.session_explorer_loading() : m.session_explorer_load_more()}
      </button>
    </div>
  {/if}
</div>

<style>
  .explorer-page {
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 20px;
    max-width: 1000px;
    margin: 0 auto;
  }

  .explorer-header {
    display: flex;
    align-items: baseline;
    gap: 10px;
  }

  .explorer-title {
    font-size: 16px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }

  .explorer-total {
    font-size: 12px;
    color: var(--text-muted);
  }

  .explorer-filters {
    display: flex;
    gap: 12px;
    flex-wrap: wrap;
  }

  .filter-field {
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-size: 10px;
    color: var(--text-muted);
  }

  .filter-field input {
    height: 26px;
    padding: 0 8px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border-muted);
    background: var(--bg-inset);
    color: var(--text-primary);
    font-size: 12px;
  }

  .search-field {
    flex: 1 1 220px;
    min-width: 180px;
  }

  .search-field input {
    width: 100%;
  }

  .load-more-row {
    display: flex;
    justify-content: center;
    padding: 8px 0;
  }

  .load-more-btn {
    padding: 6px 16px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border-default);
    background: var(--bg-surface);
    color: var(--text-primary);
    font-size: 12px;
    cursor: pointer;
  }

  .load-more-btn:disabled {
    opacity: var(--opacity-disabled);
    cursor: default;
  }
</style>
