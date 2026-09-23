<!-- ABOUTME: Session -> Prompt -> Sub-session tree for the Usage page's top sessions:
     the top level is a real ranked session (from usage.topSessions), each row lazily
     fetches GET /sessions/{id}/prompts on first expand to reveal its turns, and any
     turn that dispatched a sub-agent nests that sub-agent as another expandable
     session. Replaces the earlier ConversationTreeSpike fixture-data prototype. -->
<script lang="ts" module>
  export interface PromptFetchState {
    status: "idle" | "loading" | "error" | "ready";
    prompts?: PromptNode[];
    error?: string;
  }
</script>

<script lang="ts">
  import type { Snippet } from "svelte";
  import { usage } from "../../stores/usage.svelte.js";
  import { getSessionPrompts, type PromptNode } from "../../api/sessionPrompts.js";
  import { isAbortError } from "../../api/runtime.js";
  import { m } from "../../i18n/index.js";
  import ConversationTreeNode, { type SessionTreeNode } from "./ConversationTreeNode.svelte";

  interface Props {
    // Rows to render as top-level Session nodes. Defaults to the Usage
    // page's ranked top sessions; the full-page session explorer passes its
    // own paginated, unranked rows instead, reusing this same tree UI.
    rows?: SessionTreeNode[];
    // "" explicitly suppresses the title row (e.g. the full-page explorer,
    // which has its own page-level heading); omitted defaults to the
    // Usage-page title.
    title?: string;
    error?: string | null;
    onRetry?: () => void;
    emptyMessage?: string;
    showRank?: boolean;
    // Extra content rendered next to the title, e.g. the Usage page's
    // "View all sessions" link into the full-page session explorer.
    headerAction?: Snippet;
  }

  let {
    rows,
    title,
    error = undefined,
    onRetry,
    emptyMessage,
    showRank = true,
    headerAction,
  }: Props = $props();

  const effectiveTitle = $derived(title === undefined ? m.usage_top_sessions_prompt_breakdown() : title);
  const effectiveEmptyMessage = $derived(
    emptyMessage === undefined ? m.shared_no_sessions_in_range() : emptyMessage,
  );

  const defaultSessions = $derived<SessionTreeNode[]>(
    (usage.topSessions ?? []).map((s) => ({
      kind: "session",
      id: s.sessionId,
      sessionId: s.sessionId,
      isRoot: true,
      displayName: s.displayName || s.sessionId,
      agent: s.agent,
      project: s.project,
      startedAt: s.startedAt,
      cost: s.cost,
      tokens: {
        input: s.inputTokens,
        output: s.outputTokens,
        cacheWrite: s.cacheCreationTokens,
        cacheRead: s.cacheReadTokens,
        total: s.totalTokens,
      },
    })),
  );
  const sessions = $derived(rows ?? defaultSessions);
  const effectiveError = $derived(error === undefined ? usage.errors.topSessions : error);
  const effectiveOnRetry = $derived(onRetry ?? (() => usage.fetchTopSessions()));

  let openIds = $state<Set<string>>(new Set());
  let promptState = $state<Map<string, PromptFetchState>>(new Map());
  const abortControllers = new Map<string, AbortController>();

  function isOpen(id: string): boolean {
    return openIds.has(id);
  }

  function ensurePromptsLoaded(sessionId: string): void {
    const existing = promptState.get(sessionId);
    if (existing && (existing.status === "loading" || existing.status === "ready")) return;

    const controller = new AbortController();
    abortControllers.get(sessionId)?.abort();
    abortControllers.set(sessionId, controller);

    const next = new Map(promptState);
    next.set(sessionId, { status: "loading" });
    promptState = next;

    getSessionPrompts(sessionId, { signal: controller.signal })
      .then((prompts) => {
        const updated = new Map(promptState);
        if (!Array.isArray(prompts)) {
          updated.set(sessionId, { status: "error", error: "Unexpected response loading prompts" });
        } else {
          updated.set(sessionId, { status: "ready", prompts });
        }
        promptState = updated;
      })
      .catch((e: unknown) => {
        if (isAbortError(e)) return;
        const updated = new Map(promptState);
        updated.set(sessionId, {
          status: "error",
          error: e instanceof Error ? e.message : "Failed to load prompts",
        });
        promptState = updated;
      });
  }

  function toggleOpen(node: SessionTreeNode | { kind: "prompt"; id: string }): void {
    const next = new Set(openIds);
    const opening = !next.has(node.id);
    if (opening) {
      next.add(node.id);
    } else {
      next.delete(node.id);
    }
    openIds = next;
    if (opening && node.kind === "session") {
      ensurePromptsLoaded(node.sessionId);
    }
  }

  function retry(sessionId: string): void {
    const next = new Map(promptState);
    next.delete(sessionId);
    promptState = next;
    ensurePromptsLoaded(sessionId);
  }
</script>

<div class="conversation-tree-container">
  {#if effectiveTitle || headerAction}
    <div class="chart-title-row">
      {#if effectiveTitle}
        <h3 class="chart-title">{effectiveTitle}</h3>
      {/if}
      {#if headerAction}
        {@render headerAction()}
      {/if}
    </div>
  {/if}

  {#if effectiveError}
    <div class="error">
      {effectiveError}
      <button class="retry-btn" onclick={effectiveOnRetry}>{m.shared_retry()}</button>
    </div>
  {:else if sessions.length > 0}
    <div class="tree">
      {#each sessions as session, index (session.id)}
        <ConversationTreeNode
          node={session}
          depth={0}
          rank={showRank ? index + 1 : undefined}
          {isOpen}
          {toggleOpen}
          {promptState}
          {ensurePromptsLoaded}
          {retry}
        />
      {/each}
    </div>
  {:else}
    <div class="empty">{effectiveEmptyMessage}</div>
  {/if}
</div>

<style>
  .conversation-tree-container {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .chart-title-row {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 8px;
  }

  .chart-title {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .tree {
    display: flex;
    flex-direction: column;
    gap: 2px;
    overflow-y: auto;
  }

  .empty {
    color: var(--text-muted);
    font-size: 12px;
    padding: 24px;
    text-align: center;
  }

  .error {
    color: var(--accent-red);
    font-size: 12px;
    padding: 12px;
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .retry-btn {
    padding: 2px 8px;
    border: 1px solid currentColor;
    border-radius: var(--radius-sm);
    font-size: 11px;
    color: inherit;
    cursor: pointer;
  }
</style>
