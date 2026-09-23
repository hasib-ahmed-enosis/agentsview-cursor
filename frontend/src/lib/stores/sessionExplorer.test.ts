// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import { testMoney } from "../test/money.js";
import type { SessionExplorerPage, SessionExplorerRow } from "../api/sessionExplorer.js";

const getSessionExplorerRoots = vi.fn<
  (params: Record<string, unknown>) => Promise<SessionExplorerPage>
>();

vi.mock("../api/runtime.js", () => ({
  isAbortError: vi.fn(() => false),
}));

vi.mock("../api/sessionExplorer.js", () => ({
  getSessionExplorerRoots: (params: Record<string, unknown>) => getSessionExplorerRoots(params),
}));

const { SessionExplorerStore } = await import("./sessionExplorer.svelte.js");

function deferred<T>(): { promise: Promise<T>; resolve: (v: T) => void; reject: (e: unknown) => void } {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function row(id: string): SessionExplorerRow {
  return {
    id,
    project: "demo",
    machine: "m1",
    agent: "claude",
    display_name: id,
    started_at: "2026-07-01T00:00:00Z",
    ended_at: null,
    message_count: 1,
    usage: {
      inputTokens: 1,
      outputTokens: 1,
      cacheCreationTokens: 0,
      cacheReadTokens: 0,
      totalTokens: 2,
      cost: testMoney(0.01),
      durationSeconds: 1,
    },
  };
}

afterEach(() => {
  getSessionExplorerRoots.mockReset();
});

describe("SessionExplorerStore", () => {
  it("loads the first page on loadMore", async () => {
    getSessionExplorerRoots.mockResolvedValue({
      sessions: [row("a"), row("b")],
      total: 2,
    });

    const store = new SessionExplorerStore();
    await store.loadMore();

    expect(store.rows.map((r) => r.id)).toEqual(["a", "b"]);
    expect(store.total).toBe(2);
    expect(store.hasMore).toBe(false);
    expect(store.error).toBeNull();
  });

  it("appends subsequent pages using the returned cursor, without duplicating rows", async () => {
    getSessionExplorerRoots
      .mockResolvedValueOnce({ sessions: [row("a")], total: 2, next_cursor: "cursor-1" })
      .mockResolvedValueOnce({ sessions: [row("b")], total: 2 });

    const store = new SessionExplorerStore();
    await store.loadMore();
    expect(store.hasMore).toBe(true);

    await store.loadMore();
    expect(store.rows.map((r) => r.id)).toEqual(["a", "b"]);
    expect(getSessionExplorerRoots).toHaveBeenLastCalledWith(
      expect.objectContaining({ cursor: "cursor-1" }),
    );
    expect(store.hasMore).toBe(false);
  });

  it("resets rows and cursor on reload, e.g. after a filter change", async () => {
    getSessionExplorerRoots
      .mockResolvedValueOnce({ sessions: [row("a")], total: 1 })
      .mockResolvedValueOnce({ sessions: [row("b")], total: 1 });

    const store = new SessionExplorerStore();
    await store.loadMore();
    expect(store.rows.map((r) => r.id)).toEqual(["a"]);

    await store.reload();
    expect(store.rows.map((r) => r.id)).toEqual(["b"]);
  });

  it("a reload triggered while a loadMore is still in flight wins: the stale request never overwrites rows or leaves loading stuck", async () => {
    const first = deferred<SessionExplorerPage>();
    const second = deferred<SessionExplorerPage>();
    getSessionExplorerRoots.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);

    const store = new SessionExplorerStore();
    const firstLoad = store.loadMore(); // e.g. a "Load more" click still in flight

    // A filter change reloads while the first request hasn't resolved yet.
    const reloadPromise = store.reload();
    expect(store.loading).toBe(true);

    // The stale (pre-filter-change) request resolves after the new one has
    // already started; it must be ignored rather than appended onto rows
    // reload() already cleared, and must not clear `loading` out from under
    // the still-in-flight new request.
    first.resolve({ sessions: [row("stale")], total: 1 });
    await firstLoad;
    expect(store.loading).toBe(true);
    expect(store.rows).toEqual([]);

    second.resolve({ sessions: [row("fresh")], total: 1 });
    await reloadPromise;

    expect(store.loading).toBe(false);
    expect(store.rows.map((r) => r.id)).toEqual(["fresh"]);
  });

  it("setFilters merges into existing filters and triggers a reload", async () => {
    getSessionExplorerRoots.mockResolvedValue({ sessions: [], total: 0 });

    const store = new SessionExplorerStore();
    store.setFilters({ project: "demo" });
    await Promise.resolve();
    await Promise.resolve();

    store.setFilters({ agent: "claude" });
    await Promise.resolve();
    await Promise.resolve();

    expect(store.filters).toEqual({ project: "demo", agent: "claude", dateFrom: "", dateTo: "" });
    expect(getSessionExplorerRoots).toHaveBeenLastCalledWith(
      expect.objectContaining({ project: "demo", agent: "claude" }),
    );
  });

  it("surfaces a fetch error and leaves rows unchanged", async () => {
    getSessionExplorerRoots.mockRejectedValueOnce(new Error("boom"));

    const store = new SessionExplorerStore();
    await store.loadMore();

    expect(store.error).toBe("boom");
    expect(store.rows).toEqual([]);
  });
});
