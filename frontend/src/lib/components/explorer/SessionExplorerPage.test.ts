// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import { mount, tick, unmount } from "svelte";
import { testMoney } from "../../test/money.js";
import type { SessionExplorerPage as SessionExplorerPageResponse } from "../../api/sessionExplorer.js";

const getSessionExplorerRoots = vi.fn<
  (params: Record<string, unknown>) => Promise<SessionExplorerPageResponse>
>();

vi.mock("../../api/runtime.js", () => ({
  isAbortError: vi.fn(() => false),
}));

vi.mock("../../api/sessionExplorer.js", () => ({
  getSessionExplorerRoots: (params: Record<string, unknown>) => getSessionExplorerRoots(params),
}));

vi.mock("../../stores/sessions.svelte.js", () => ({
  sessions: { filters: { project: "", agent: "" } },
}));

const { sessionExplorer } = await import("../../stores/sessionExplorer.svelte.js");
const SessionExplorerPage = (await import("./SessionExplorerPage.svelte")).default;

let component: ReturnType<typeof mount> | undefined;

function usage(overrides: Partial<SessionExplorerPageResponse["sessions"][number]["usage"]> = {}) {
  return {
    inputTokens: 0,
    outputTokens: 0,
    cacheCreationTokens: 0,
    cacheReadTokens: 0,
    totalTokens: 0,
    cost: testMoney(0),
    durationSeconds: 0,
    ...overrides,
  };
}

afterEach(() => {
  if (component) {
    unmount(component);
    component = undefined;
  }
  sessionExplorer.rows = [];
  sessionExplorer.total = 0;
  sessionExplorer.nextCursor = undefined;
  sessionExplorer.error = null;
  getSessionExplorerRoots.mockReset();
  document.body.innerHTML = "";
});

describe("SessionExplorerPage", () => {
  it("loads and renders a page of root sessions on mount", async () => {
    getSessionExplorerRoots.mockResolvedValue({
      sessions: [
        {
          id: "root-1",
          project: "demo",
          machine: "m1",
          agent: "claude",
          display_name: "Root conversation",
          started_at: "2026-07-01T00:00:00Z",
          ended_at: null,
          message_count: 3,
          usage: usage({ totalTokens: 42, cost: testMoney(0.25) }),
        },
      ],
      total: 1,
    });

    component = mount(SessionExplorerPage, { target: document.body });
    await tick();
    await tick();
    await tick();

    expect(document.querySelector(".conversation-label")?.textContent?.trim()).toBe(
      "Root conversation",
    );
    expect(document.querySelector(".explorer-total")?.textContent?.trim()).toBe("1 session");
    expect(document.querySelector(".row-cost")?.textContent?.trim()).toBe("$0.25");
  });

  it("shows a Load more button only while a next cursor exists, and fetches the next page on click", async () => {
    getSessionExplorerRoots
      .mockResolvedValueOnce({
        sessions: [
          {
            id: "root-1",
            project: "demo",
            machine: "m1",
            agent: "claude",
            display_name: "First",
            started_at: "2026-07-01T00:00:00Z",
            ended_at: null,
            message_count: 1,
            usage: usage(),
          },
        ],
        total: 2,
        next_cursor: "cursor-1",
      })
      .mockResolvedValueOnce({
        sessions: [
          {
            id: "root-2",
            project: "demo",
            machine: "m1",
            agent: "claude",
            display_name: "Second",
            started_at: "2026-07-01T01:00:00Z",
            ended_at: null,
            message_count: 1,
            usage: usage(),
          },
        ],
        total: 2,
      });

    component = mount(SessionExplorerPage, { target: document.body });
    await tick();
    await tick();
    await tick();

    const loadMoreBtn = document.querySelector<HTMLButtonElement>(".load-more-btn");
    expect(loadMoreBtn).not.toBeNull();

    loadMoreBtn?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await tick();
    await tick();
    await tick();

    const labels = [...document.querySelectorAll(".conversation-label")].map((el) =>
      el.textContent?.trim(),
    );
    expect(labels).toEqual(["First", "Second"]);
    expect(document.querySelector(".load-more-btn")).toBeNull();
  });

  it("shows the empty state when no sessions match the current filters", async () => {
    getSessionExplorerRoots.mockResolvedValue({ sessions: [], total: 0 });

    component = mount(SessionExplorerPage, { target: document.body });
    await tick();
    await tick();
    await tick();

    expect(document.querySelector(".empty")?.textContent?.trim()).toBe(
      "No sessions match these filters",
    );
  });
});
