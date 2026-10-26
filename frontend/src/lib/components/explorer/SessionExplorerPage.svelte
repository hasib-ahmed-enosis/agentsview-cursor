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
  import ProjectTypeahead from "../layout/ProjectTypeahead.svelte";
  import AgentTypeahead from "../layout/AgentTypeahead.svelte";
  import { compareMoney } from "../../money.js";

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

  // Client-side sort over the rows loaded so far (like the search box
  // above, this only reorders what "Load more" has fetched — it does not
  // re-query the server).
  type SortKey =
    | "name"
    | "date"
    | "cost"
    | "total_tokens"
    | "input_tokens"
    | "output_tokens"
    | "cache_write_tokens"
    | "cache_read_tokens";

  let sortKey = $state<SortKey>("date");
  let sortDir = $state<"asc" | "desc">("desc");

  function compareRows(a: (typeof sessionExplorer.rows)[number], b: (typeof sessionExplorer.rows)[number], key: SortKey): number {
    switch (key) {
      case "name":
        return displayNameFor(a).toLowerCase().localeCompare(displayNameFor(b).toLowerCase());
      case "date":
        return (a.started_at ? Date.parse(a.started_at) : 0) - (b.started_at ? Date.parse(b.started_at) : 0);
      case "cost":
        return compareMoney(a.usage.cost, b.usage.cost);
      case "total_tokens":
        return a.usage.totalTokens - b.usage.totalTokens;
      case "input_tokens":
        return a.usage.inputTokens - b.usage.inputTokens;
      case "output_tokens":
        return a.usage.outputTokens - b.usage.outputTokens;
      case "cache_write_tokens":
        return a.usage.cacheCreationTokens - b.usage.cacheCreationTokens;
      case "cache_read_tokens":
        return a.usage.cacheReadTokens - b.usage.cacheReadTokens;
    }
  }

  const sortedRows = $derived(
    [...filteredRows].sort((a, b) => {
      const cmp = compareRows(a, b, sortKey);
      return sortDir === "asc" ? cmp : -cmp;
    }),
  );

  function toggleSortDir(): void {
    sortDir = sortDir === "asc" ? "desc" : "asc";
  }

  const rows = $derived<SessionTreeNode[]>(
    sortedRows.map((row) => ({
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
        hasUsageData: row.usage.hasUsageData,
      },
    })),
  );

  // Export the rows currently shown (after search + sort), as CSV or JSON.
  let exportOpen = $state(false);

  function download(filename: string, mime: string, content: string): void {
    const url = URL.createObjectURL(new Blob([content], { type: mime }));
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  }

  function csvCell(value: unknown): string {
    let s = String(value ?? "");
    // Neutralise spreadsheet formula injection.
    if (/^[=+\-@\t\r]/.test(s)) s = `'${s}`;
    return /[",\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
  }

  function exportRows(format: "csv" | "json"): void {
    exportOpen = false;
    const data = sortedRows.map((row) => ({
      id: row.id,
      name: displayNameFor(row),
      project: row.project,
      agent: row.agent,
      started_at: row.started_at ?? "",
      relationship_type: row.relationship_type ?? "",
      cost: row.usage.cost,
      total_tokens: row.usage.totalTokens,
      input_tokens: row.usage.inputTokens,
      output_tokens: row.usage.outputTokens,
      cache_write_tokens: row.usage.cacheCreationTokens,
      cache_read_tokens: row.usage.cacheReadTokens,
    }));
    const stamp = new Date().toISOString().slice(0, 10);
    if (format === "json") {
      download(`session-explorer-${stamp}.json`, "application/json", JSON.stringify(data, null, 2));
      return;
    }
    const headers = Object.keys(data[0] ?? {
      id: 0, name: 0, project: 0, agent: 0, started_at: 0, relationship_type: 0, cost: 0,
      total_tokens: 0, input_tokens: 0, output_tokens: 0, cache_write_tokens: 0, cache_read_tokens: 0,
    });
    const lines = [
      headers.join(","),
      ...data.map((d) =>
        headers
          .map((h) => {
            const v = (d as Record<string, unknown>)[h];
            return csvCell(typeof v === "object" && v !== null ? JSON.stringify(v) : v);
          })
          .join(","),
      ),
    ];
    download(`session-explorer-${stamp}.csv`, "text/csv;charset=utf-8", "﻿" + lines.join("\r\n"));
  }

  onMount(() => {
    sessions.loadProjects();
    sessions.loadAgents();
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
    <div class="export-menu">
      <button
        type="button"
        class="export-btn"
        disabled={sortedRows.length === 0}
        aria-haspopup="menu"
        aria-expanded={exportOpen}
        onclick={() => (exportOpen = !exportOpen)}
      >
        {m.session_explorer_export()} ▾
      </button>
      {#if exportOpen}
        <div class="export-dropdown" role="menu">
          <button type="button" role="menuitem" onclick={() => exportRows("csv")}>{m.session_explorer_export_csv()}</button>
          <button type="button" role="menuitem" onclick={() => exportRows("json")}>{m.session_explorer_export_json()}</button>
        </div>
      {/if}
    </div>
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
      <ProjectTypeahead
        projects={sessions.projects}
        value={sessionExplorer.filters.project}
        placeholder={m.session_explorer_filter_project_placeholder()}
        onselect={(v) => sessionExplorer.setFilters({ project: v })}
      />
    </label>
    <label class="filter-field">
      <span>{m.session_explorer_filter_agent()}</span>
      <AgentTypeahead
        agents={sessions.agents}
        value={sessionExplorer.filters.agent}
        placeholder={m.session_explorer_filter_agent_placeholder()}
        onselect={(v) => sessionExplorer.setFilters({ agent: v })}
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
    <label class="filter-field">
      <span>{m.session_explorer_sort_label()}</span>
      <div class="sort-control">
        <select
          value={sortKey}
          onchange={(e) => (sortKey = e.currentTarget.value as SortKey)}
        >
          <option value="date">{m.session_explorer_sort_date()}</option>
          <option value="name">{m.session_explorer_sort_name()}</option>
          <option value="cost">{m.session_explorer_sort_cost()}</option>
          <option value="total_tokens">{m.session_explorer_sort_total_tokens()}</option>
          <option value="input_tokens">{m.session_explorer_sort_input_tokens()}</option>
          <option value="output_tokens">{m.session_explorer_sort_output_tokens()}</option>
          <option value="cache_write_tokens">{m.session_explorer_sort_cache_write_tokens()}</option>
          <option value="cache_read_tokens">{m.session_explorer_sort_cache_read_tokens()}</option>
        </select>
        <button
          type="button"
          class="sort-dir-btn"
          onclick={toggleSortDir}
          title={sortDir === "asc" ? m.session_explorer_sort_ascending() : m.session_explorer_sort_descending()}
          aria-label={sortDir === "asc" ? m.session_explorer_sort_ascending() : m.session_explorer_sort_descending()}
        >
          {sortDir === "asc" ? "↑" : "↓"}
        </button>
      </div>
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

  .export-menu {
    position: relative;
    margin-left: auto;
  }

  .export-btn,
  .export-dropdown button {
    height: 26px;
    padding: 0 10px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border-muted);
    background: var(--bg-inset);
    color: var(--text-primary);
    font-size: 12px;
    cursor: pointer;
    white-space: nowrap;
  }

  .export-btn:disabled {
    opacity: var(--opacity-disabled);
    cursor: default;
  }

  .export-btn:hover:not(:disabled),
  .export-dropdown button:hover {
    background: var(--bg-surface-hover);
  }

  .export-dropdown {
    position: absolute;
    right: 0;
    top: 30px;
    z-index: 10;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 4px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border-default);
    background: var(--bg-surface);
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

  .filter-field :global(.kit-typeahead) {
    --typeahead-min-width: 160px;
  }

  .sort-control {
    display: flex;
    gap: 4px;
  }

  .sort-control select {
    height: 26px;
    padding: 0 6px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border-muted);
    background: var(--bg-inset);
    color: var(--text-primary);
    font-size: 12px;
  }

  .sort-dir-btn {
    height: 26px;
    width: 26px;
    flex-shrink: 0;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border-muted);
    background: var(--bg-inset);
    color: var(--text-primary);
    font-size: 12px;
    line-height: 1;
    cursor: pointer;
  }

  .sort-dir-btn:hover {
    background: var(--bg-surface-hover);
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
