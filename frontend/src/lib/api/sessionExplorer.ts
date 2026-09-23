// Hand-written client for GET /api/v1/sessions/explorer/roots
// (internal/server/huma_routes_sessions.go, humaGetSessionExplorerRoots).
//
// This mirrors the shape Orval would generate from openapi.yaml, but the
// route was added after this environment's DuckDB/CGO toolchain broke, so
// `npm run generate:api` can't be run here — see sessionPrompts.ts for the
// same note. Once codegen runs successfully, this file's
// `SessionExplorerRow`/`SessionExplorerPage` types and `getSessionExplorerRoots`
// should be replaced with the generated equivalents.
import { orvalFetch, type ApiRequestOptions } from "./runtime.js";
import type { PromptUsage } from "./sessionPrompts.js";

// SessionExplorerRow mirrors internal/db.SessionExplorerRow: a session's own
// (snake_case, matching every other session JSON payload in this codebase)
// metadata, plus its own+descendant rolled-up usage.
export interface SessionExplorerRow {
  id: string;
  project: string;
  machine: string;
  agent: string;
  display_name?: string;
  first_message?: string | null;
  started_at: string | null;
  ended_at: string | null;
  message_count: number;
  parent_session_id?: string;
  relationship_type?: string;
  usage: PromptUsage;
}

export interface SessionExplorerPage {
  sessions: SessionExplorerRow[];
  next_cursor?: string;
  total: number;
}

export interface SessionExplorerRootsParams {
  cursor?: string;
  limit?: number;
  project?: string;
  agent?: string;
  date_from?: string;
  date_to?: string;
}

export function getSessionExplorerRoots(
  params: SessionExplorerRootsParams = {},
  options?: ApiRequestOptions,
): Promise<SessionExplorerPage> {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") query.set(key, String(value));
  }
  const qs = query.toString();
  return orvalFetch<SessionExplorerPage>(
    `/api/v1/sessions/explorer/roots${qs ? `?${qs}` : ""}`,
    { ...options, method: "GET" },
  );
}
