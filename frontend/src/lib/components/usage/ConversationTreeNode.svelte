<!-- ABOUTME: Recursive row renderer for ConversationTree.svelte. A "session" node
     (top-level top-session or a nested sub-agent) lazily fetches its own Prompt
     children on first expand; a "prompt" node's children are the sub-agent
     sessions it dispatched, each rendered as another expandable session node. -->
<script lang="ts" module>
  import type { Money } from "../../money.js";
  import type { PromptNode, SubSessionRef } from "../../api/sessionPrompts.js";

  export interface SessionTreeNode {
    kind: "session";
    id: string;
    sessionId: string;
    isRoot: boolean;
    displayName: string;
    agent?: string;
    project?: string;
    startedAt?: string;
    role?: string;
    relationshipType?: string;
    // Top-level rows get this from usage.topSessions; nested sub-session
    // rows get it from their parent Prompt's SubSessionRef.usage, already
    // rolled up (own activity plus every descendant's) by the backend.
    cost?: Money;
    tokens?: { input: number; output: number; cacheWrite: number; cacheRead: number; total: number; hasUsageData?: boolean };
  }

  export interface PromptTreeNode {
    kind: "prompt";
    id: string;
    ordinal: number;
    startedAt: string;
    messageCount: number;
    subSessions: SubSessionRef[];
    cost: Money;
    tokens: { input: number; output: number; cacheWrite: number; cacheRead: number; total: number; hasUsageData?: boolean };
    isContinuationStart?: boolean;
    text?: string;
  }

  export type ConversationTreeNodeData = SessionTreeNode | PromptTreeNode;
</script>

<script lang="ts">
  import type { PromptFetchState } from "./ConversationTree.svelte";
  import Self from "./ConversationTreeNode.svelte";
  import { formatAgentName, formatTokenCount, formatTimestamp } from "../../utils/format.js";
  import { formatMoney } from "../../money.js";
  import { ChevronDownIcon, ChevronRightIcon } from "../../icons.js";
  import { m } from "../../i18n/index.js";
  import { router } from "../../stores/router.svelte.js";

  interface Props {
    node: ConversationTreeNodeData;
    depth: number;
    rank?: number;
    isOpen: (id: string) => boolean;
    toggleOpen: (node: SessionTreeNode | { kind: "prompt"; id: string }) => void;
    promptState: Map<string, PromptFetchState>;
    ensurePromptsLoaded: (sessionId: string) => void;
    retry: (sessionId: string) => void;
  }

  let { node, depth, rank, isOpen, toggleOpen, promptState, ensurePromptsLoaded, retry }: Props =
    $props();

  const open = $derived(isOpen(node.id));
  const isTopRow = $derived(depth === 0);

  // A prompt node's children are known up front (its dispatched sub-sessions);
  // a session node's children come from its own lazy prompt fetch.
  const fetchState = $derived(node.kind === "session" ? promptState.get(node.sessionId) : undefined);
  const promptChildren = $derived<PromptTreeNode[]>(
    node.kind === "session" && fetchState?.status === "ready"
      ? (fetchState.prompts ?? []).map((p: PromptNode) => ({
          kind: "prompt" as const,
          id: p.id,
          ordinal: p.ordinal,
          startedAt: p.startedAt,
          messageCount: p.messageCount,
          subSessions: p.subSessions ?? [],
          isContinuationStart: p.isContinuationStart,
          text: p.text,
          cost: p.usage.cost,
          tokens: {
            input: p.usage.inputTokens,
            output: p.usage.outputTokens,
            cacheWrite: p.usage.cacheCreationTokens,
            cacheRead: p.usage.cacheReadTokens,
            total: p.usage.totalTokens,
            hasUsageData: p.usage.hasUsageData,
          },
        }))
      : [],
  );
  const subSessionChildren = $derived<SessionTreeNode[]>(
    node.kind === "prompt"
      ? node.subSessions.map((s) => ({
          kind: "session" as const,
          id: `${node.id}:${s.sessionId}`,
          sessionId: s.sessionId,
          isRoot: false,
          displayName: s.displayName || s.sessionId,
          agent: s.agent,
          startedAt: s.startedAt,
          role: s.role,
          relationshipType: s.relationshipType,
          cost: s.usage.cost,
          tokens: {
            input: s.usage.inputTokens,
            output: s.usage.outputTokens,
            cacheWrite: s.usage.cacheCreationTokens,
            cacheRead: s.usage.cacheReadTokens,
            total: s.usage.totalTokens,
            hasUsageData: s.usage.hasUsageData,
          },
        }))
      : [],
  );

  const hasKnownChildren = $derived(
    node.kind === "prompt" ? node.subSessions.length > 0 : true,
  );

  const tokens = $derived(node.kind === "session" ? node.tokens : node.tokens);
  // Some providers (e.g. Cursor) never record per-message token usage at
  // all, so their totals are unknown rather than genuinely zero — shown as
  // "x" instead of a misleading "0".
  function tokenLabel(value: number, hasUsageData: boolean | undefined): string {
    return hasUsageData === false ? "x" : formatTokenCount(value);
  }
  const tokenBreakdown = $derived(
    tokens
      ? m.usage_tree_token_breakdown({
          input: tokenLabel(tokens.input, tokens.hasUsageData),
          output: tokenLabel(tokens.output, tokens.hasUsageData),
          cacheWrite: tokenLabel(tokens.cacheWrite, tokens.hasUsageData),
          cacheRead: tokenLabel(tokens.cacheRead, tokens.hasUsageData),
        })
      : undefined,
  );

  function handleToggle(): void {
    toggleOpen(node.kind === "session" ? node : { kind: "prompt", id: node.id });
  }

  function handleNavigate(): void {
    if (node.kind === "session") {
      router.navigateToSession(node.sessionId);
    }
  }

  const typeLabel = $derived(
    node.kind === "prompt"
      ? m.usage_tree_tag_prompt()
      : node.isRoot
        ? m.usage_tree_tag_session()
        : m.usage_tree_tag_subagent(),
  );
</script>

<div class="row" style="--depth: {depth}">
  {#if isTopRow && rank !== undefined}
    <span class="rank">{rank}</span>
  {:else}
    <span class="chevron chevron-spacer"></span>
  {/if}

  {#if hasKnownChildren}
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <span class="chevron chevron-inline" onclick={handleToggle}>
      {#if open}
        <ChevronDownIcon size="12" strokeWidth="2.2" aria-hidden="true" />
      {:else}
        <ChevronRightIcon size="12" strokeWidth="2.2" aria-hidden="true" />
      {/if}
    </span>
  {:else}
    <span class="chevron chevron-spacer"></span>
  {/if}

  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="conversation-info"
    class:conversation-info-clickable={node.kind === "session"}
    onclick={handleNavigate}
  >
    <span class="conversation-label">
      {#if node.kind === "session"}
        {node.displayName}
      {:else}
        {node.text || m.usage_tree_turn_label({ ordinal: node.ordinal + 1 })}
      {/if}
    </span>
    <span class="tag-row">
      <span
        class="tag tag-type"
        class:tag-session={node.kind === "session" && node.isRoot}
        class:tag-subagent={node.kind === "session" && !node.isRoot}
        class:tag-prompt={node.kind === "prompt"}
      >
        {typeLabel}
      </span>
      {#if node.kind === "prompt" && node.isContinuationStart}
        <span class="tag tag-continuation">{m.usage_tree_tag_continuation()}</span>
      {/if}
      {#if node.kind === "session" && node.role}
        <span class="tag tag-role">{node.role}</span>
      {/if}
      {#if node.kind === "session" && node.agent}
        <span class="tag tag-agent">{formatAgentName(node.agent)}</span>
      {/if}
      {#if node.kind === "session"}
        <span class="conversation-meta">
          {#if node.project}{node.project} &middot; {/if}{node.sessionId}
        </span>
      {:else}
        <span class="conversation-meta">
          {m.usage_tree_turn_label({ ordinal: node.ordinal + 1 })} &middot; {m.usage_tree_message_count({ count: node.messageCount })}
        </span>
      {/if}
    </span>
    {#if tokenBreakdown}
      <span class="token-breakdown">{tokenBreakdown}</span>
    {/if}
  </div>

  {#if node.kind === "session" && node.startedAt}
    <span class="row-metric">{formatTimestamp(node.startedAt)}</span>
  {:else if node.kind === "prompt"}
    <span class="row-metric">{formatTimestamp(node.startedAt)}</span>
  {/if}
  {#if node.kind === "session" && node.tokens}
    <span class="row-tokens">{tokenLabel(node.tokens.total, node.tokens.hasUsageData)}</span>
  {:else if node.kind === "prompt"}
    <span class="row-tokens">{tokenLabel(node.tokens.total, node.tokens.hasUsageData)}</span>
  {/if}
  {#if node.kind === "session" && node.cost}
    <span class="row-cost">{formatMoney(node.cost)}</span>
  {:else if node.kind === "prompt"}
    <span class="row-cost">{formatMoney(node.cost)}</span>
  {/if}
</div>

{#if open}
  {#if node.kind === "session"}
    {#if fetchState?.status === "loading"}
      <div class="status-row" style="--depth: {depth + 1}">{m.usage_tree_loading_prompts()}</div>
    {:else if fetchState?.status === "error"}
      <div class="status-row status-error" style="--depth: {depth + 1}">
        {fetchState.error}
        <button class="retry-btn" onclick={() => retry(node.sessionId)}>{m.shared_retry()}</button>
      </div>
    {:else if fetchState?.status === "ready" && promptChildren.length === 0}
      <div class="status-row" style="--depth: {depth + 1}">{m.usage_tree_no_prompts_found()}</div>
    {:else}
      {#each promptChildren as child, i (child.id)}
        <Self
          node={child}
          depth={depth + 1}
          rank={i + 1}
          {isOpen}
          {toggleOpen}
          {promptState}
          {ensurePromptsLoaded}
          {retry}
        />
      {/each}
    {/if}
  {:else}
    {#each subSessionChildren as child, i (child.id)}
      <Self
        node={child}
        depth={depth + 1}
        rank={i + 1}
        {isOpen}
        {toggleOpen}
        {promptState}
        {ensurePromptsLoaded}
        {retry}
      />
    {/each}
  {/if}
{/if}

<style>
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 6px;
    padding-left: calc(6px + var(--depth) * 20px);
    border-radius: var(--radius-sm);
    cursor: default;
    transition: background 0.1s;
  }

  .row:hover {
    background: var(--bg-surface-hover);
  }

  .status-row {
    padding: 4px 6px;
    padding-left: calc(6px + var(--depth) * 20px + 20px);
    font-size: 10px;
    color: var(--text-muted);
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .status-error {
    color: var(--accent-red);
  }

  .retry-btn {
    padding: 1px 6px;
    border: 1px solid currentColor;
    border-radius: var(--radius-sm);
    font-size: 10px;
    color: inherit;
    cursor: pointer;
  }

  .rank {
    flex-shrink: 0;
    width: 14px;
    text-align: right;
    font-size: 9px;
    font-weight: 600;
    color: var(--text-muted);
    font-family: var(--font-mono);
  }

  .chevron {
    flex-shrink: 0;
    width: 14px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-muted);
  }

  .chevron-inline {
    cursor: pointer;
  }

  .chevron-spacer {
    width: 14px;
  }

  .conversation-info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }

  .conversation-info-clickable {
    cursor: pointer;
  }

  .conversation-label {
    font-size: 11px;
    color: var(--text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .conversation-meta {
    font-size: 9px;
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .token-breakdown {
    font-size: 9px;
    font-family: var(--font-mono);
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .tag-row {
    display: flex;
    align-items: center;
    gap: 5px;
    min-width: 0;
  }

  .tag {
    flex-shrink: 0;
    font-size: 8px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.03em;
    padding: 1px 5px;
    border-radius: 4px;
    line-height: 1.4;
  }

  .tag-session {
    background: color-mix(in srgb, var(--text-primary) 16%, transparent);
    color: var(--text-primary);
  }

  .tag-prompt {
    background: color-mix(in srgb, var(--accent-blue) 16%, transparent);
    color: var(--accent-blue);
  }

  .tag-subagent {
    background: color-mix(in srgb, var(--accent-purple, var(--accent-blue)) 16%, transparent);
    color: var(--accent-purple, var(--accent-blue));
  }

  .tag-continuation {
    background: color-mix(in srgb, var(--accent-amber, var(--accent-blue)) 16%, transparent);
    color: var(--accent-amber, var(--accent-blue));
  }

  .tag-role {
    background: var(--bg-inset);
    color: var(--text-secondary);
    font-style: italic;
    text-transform: none;
  }

  .tag-agent {
    background: var(--bg-inset);
    color: var(--text-secondary);
  }

  .row-tokens {
    flex-shrink: 0;
    font-size: 10px;
    font-family: var(--font-mono);
    color: var(--text-muted);
    min-width: 36px;
    text-align: right;
  }

  .row-cost {
    flex-shrink: 0;
    font-size: 11px;
    font-weight: 500;
    font-family: var(--font-mono);
    color: var(--accent-blue);
    min-width: 48px;
    text-align: right;
  }

  .row-metric {
    flex-shrink: 0;
    font-size: 9px;
    font-family: var(--font-mono);
    color: var(--text-muted);
    min-width: 70px;
    text-align: right;
  }
</style>
