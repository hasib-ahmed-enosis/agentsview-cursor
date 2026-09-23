// Paginated store backing the full-page session explorer: fetches
// GET /api/v1/sessions/explorer/roots page by page ("Load more", matching
// the Activity page's SessionsTable/sessions_next_cursor precedent) rather
// than the Usage page's fixed top-N ranked list.
import {
  getSessionExplorerRoots,
  type SessionExplorerRow,
} from "../api/sessionExplorer.js";
import { isAbortError } from "../api/runtime.js";

export interface SessionExplorerFilters {
  project: string;
  agent: string;
  dateFrom: string;
  dateTo: string;
}

function emptyFilters(): SessionExplorerFilters {
  return { project: "", agent: "", dateFrom: "", dateTo: "" };
}

const PAGE_SIZE = 25;

export class SessionExplorerStore {
  rows: SessionExplorerRow[] = $state([]);
  total = $state(0);
  nextCursor = $state<string | undefined>(undefined);
  loading = $state(false);
  error = $state<string | null>(null);
  filters: SessionExplorerFilters = $state(emptyFilters());

  #abortController: AbortController | undefined;
  // Bumped on every loadMore call and checked before a response is applied,
  // so a request superseded by a newer one (e.g. reload() firing while a
  // "Load more" fetch is still in flight) can never overwrite state with
  // stale-filter results or clear `loading` out from under the request that
  // actually superseded it — aborting alone isn't enough, since the aborted
  // request's own `finally` still runs and would otherwise race the winner.
  #requestId = 0;

  setFilters(filters: Partial<SessionExplorerFilters>): void {
    this.filters = { ...this.filters, ...filters };
    void this.reload();
  }

  async reload(): Promise<void> {
    this.rows = [];
    this.nextCursor = undefined;
    this.total = 0;
    await this.loadMore({ force: true });
  }

  async loadMore(opts: { force?: boolean } = {}): Promise<void> {
    if (this.loading && !opts.force) return;
    this.#abortController?.abort();
    const controller = new AbortController();
    this.#abortController = controller;
    const requestId = ++this.#requestId;

    this.loading = true;
    this.error = null;
    try {
      const page = await getSessionExplorerRoots(
        {
          limit: PAGE_SIZE,
          cursor: this.nextCursor,
          project: this.filters.project || undefined,
          agent: this.filters.agent || undefined,
          date_from: this.filters.dateFrom || undefined,
          date_to: this.filters.dateTo || undefined,
        },
        { signal: controller.signal },
      );
      if (requestId !== this.#requestId) return;
      this.rows = [...this.rows, ...page.sessions];
      this.total = page.total;
      this.nextCursor = page.next_cursor || undefined;
    } catch (e: unknown) {
      if (isAbortError(e) || requestId !== this.#requestId) return;
      this.error = e instanceof Error ? e.message : "Failed to load sessions";
    } finally {
      if (requestId === this.#requestId) this.loading = false;
    }
  }

  get hasMore(): boolean {
    return this.nextCursor !== undefined;
  }
}

export const sessionExplorer = new SessionExplorerStore();
