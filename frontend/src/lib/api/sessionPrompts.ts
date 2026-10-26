// Hand-written client for GET /api/v1/sessions/{id}/prompts
// (internal/server/huma_routes_sessions.go, humaGetSessionPrompts).
//
// This mirrors the shape Orval would generate from openapi.yaml, but the
// route was added after this environment's DuckDB/CGO toolchain broke, so
// `npm run generate:api` (which shells out to the Go binary to regenerate
// openapi.yaml) can't be run here. Once codegen runs successfully, this
// file's `PromptNode`/`SubSessionRef` types and `getSessionPrompts` should
// be replaced with the generated `DbPromptNode` type and
// `SessionsService.getApiV1SessionsByIdPrompts`.
import { orvalFetch, type ApiRequestOptions } from "./runtime.js";
import type { Money } from "../money.js";

// PromptUsage is a token/cost/duration rollup for one tree node: its own
// activity plus the sum of every descendant's already-rolled-up totals (a
// flame-graph rule), so a node's numbers are correct before its subtree is
// ever expanded.
export interface PromptUsage {
  inputTokens: number;
  outputTokens: number;
  cacheCreationTokens: number;
  cacheReadTokens: number;
  totalTokens: number;
  cost: Money;
  durationSeconds: number;
  // False when none of this node's own messages (nor any descendant's)
  // ever carried a parseable token-usage payload at all — as opposed to a
  // node whose messages reported genuinely zero tokens. Some providers
  // (e.g. Cursor) never record per-message usage, so their totals are
  // unknown rather than zero. Optional/absent is treated as true (has
  // data) so fixtures/callers that predate this field keep behaving as
  // before.
  hasUsageData?: boolean;
}

export interface SubSessionRef {
  sessionId: string;
  role?: string;
  displayName?: string;
  agent?: string;
  startedAt?: string;
  relationshipType?: string;
  usage: PromptUsage;
}

export interface PromptNode {
  id: string;
  sessionId: string;
  ordinal: number;
  startedAt: string;
  messageCount: number;
  usage: PromptUsage;
  subSessions?: SubSessionRef[];
  // True only for the first Prompt of a session that continues an earlier
  // one (relationship_type "continuation"): the merged Session/Tab concept
  // keeps a continuation as more Prompts in the same list rather than a
  // separate root, tagging only the hand-off point.
  isContinuationStart?: boolean;
  // A truncated excerpt (180 chars) of the real user turn's own content —
  // the actual ask. Empty/absent for a Prompt whose triggering message has
  // no text content (e.g. an image-only turn).
  text?: string;
}

export function getSessionPrompts(
  sessionId: string,
  options?: ApiRequestOptions,
): Promise<PromptNode[]> {
  return orvalFetch<PromptNode[]>(`/api/v1/sessions/${encodeURIComponent(sessionId)}/prompts`, {
    ...options,
    method: "GET",
  });
}
