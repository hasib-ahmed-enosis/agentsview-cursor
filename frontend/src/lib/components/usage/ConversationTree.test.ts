// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vite-plus/test";
import { mount, tick, unmount } from "svelte";
import { usage } from "../../stores/usage.svelte.js";
import { router } from "../../stores/router.svelte.js";
import { testMoney } from "../../test/money.js";
import type { PromptNode } from "../../api/sessionPrompts.js";

const getSessionPrompts = vi.fn<(sessionId: string) => Promise<PromptNode[]>>();

vi.mock("../../api/runtime.js", () => ({
  isAbortError: vi.fn(() => false),
}));

vi.mock("../../api/sessionPrompts.js", () => ({
  getSessionPrompts: (sessionId: string) => getSessionPrompts(sessionId),
}));

import ConversationTree from "./ConversationTree.svelte";

let component: ReturnType<typeof mount> | undefined;

function deferred<T>(): { promise: Promise<T>; resolve: (v: T) => void; reject: (e: unknown) => void } {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

afterEach(() => {
  if (component) {
    unmount(component);
    component = undefined;
  }
  usage.topSessions = null;
  usage.errors.topSessions = null;
  getSessionPrompts.mockReset();
  document.body.innerHTML = "";
});

describe("ConversationTree", () => {
  it("renders top sessions as root rows", async () => {
    usage.topSessions = [
      {
        sessionId: "root-1",
        displayName: "Root conversation",
        agent: "claude",
        project: "demo",
        startedAt: "2026-07-01T00:00:00Z",
        inputTokens: 10,
        outputTokens: 5,
        cacheCreationTokens: 0,
        cacheReadTokens: 0,
        totalTokens: 15,
        cost: testMoney(1),
      },
    ];

    component = mount(ConversationTree, { target: document.body });
    await tick();

    expect(document.querySelector(".conversation-label")?.textContent?.trim()).toBe(
      "Root conversation",
    );
    expect(document.querySelector(".tag-session")?.textContent?.trim()).toBe("Session");
    expect(document.querySelector(".row-cost")?.textContent?.trim()).toBe("$1.00");
    expect(document.querySelector(".token-breakdown")?.textContent?.trim()).toBe(
      "in 10 · out 5 · cache write 0 · cache read 0",
    );
  });

  it("clicking a session row navigates to that session's detail page", async () => {
    usage.topSessions = [
      {
        sessionId: "root-1",
        displayName: "Root conversation",
        agent: "claude",
        project: "demo",
        startedAt: "2026-07-01T00:00:00Z",
        inputTokens: 0,
        outputTokens: 0,
        cacheCreationTokens: 0,
        cacheReadTokens: 0,
        totalTokens: 0,
        cost: testMoney(0),
      },
    ];
    const navigateToSession = vi.spyOn(router, "navigateToSession").mockImplementation(() => {});

    component = mount(ConversationTree, { target: document.body });
    await tick();

    document
      .querySelector<HTMLElement>(".conversation-info")
      ?.dispatchEvent(new MouseEvent("click", { bubbles: true }));

    expect(navigateToSession).toHaveBeenCalledExactlyOnceWith("root-1");
    navigateToSession.mockRestore();
  });

  it("renders the empty state when there are no top sessions", async () => {
    usage.topSessions = null;

    component = mount(ConversationTree, { target: document.body });
    await tick();

    expect(document.querySelector(".empty")?.textContent?.trim()).toBe("No sessions in range");
  });

  it("shows the top-sessions error state and retries the fetch on click", async () => {
    const fetchTopSessions = vi.spyOn(usage, "fetchTopSessions").mockResolvedValue("ok");
    usage.errors.topSessions = "network error";

    component = mount(ConversationTree, { target: document.body });
    await tick();

    expect(document.querySelector(".error")?.textContent).toContain("network error");
    document
      .querySelector<HTMLElement>(".error .retry-btn")
      ?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await tick();

    expect(fetchTopSessions).toHaveBeenCalledOnce();
  });

  it("lazily fetches a session's prompts on first expand, showing a loading state first", async () => {
    usage.topSessions = [
      {
        sessionId: "root-1",
        displayName: "Root conversation",
        agent: "claude",
        project: "demo",
        startedAt: "2026-07-01T00:00:00Z",
        inputTokens: 0,
        outputTokens: 0,
        cacheCreationTokens: 0,
        cacheReadTokens: 0,
        totalTokens: 0,
        cost: testMoney(0),
      },
    ];
    const gate = deferred<PromptNode[]>();
    getSessionPrompts.mockReturnValue(gate.promise);

    component = mount(ConversationTree, { target: document.body });
    await tick();

    document
      .querySelector<HTMLElement>(".chevron-inline")
      ?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await tick();

    expect(getSessionPrompts).toHaveBeenCalledExactlyOnceWith("root-1");
    expect(document.querySelector(".status-row")?.textContent?.trim()).toBe("Loading prompts…");

    gate.resolve([
      {
        id: "root-1:0",
        sessionId: "root-1",
        ordinal: 0,
        startedAt: "2026-07-01T00:00:01Z",
        messageCount: 2,
        usage: {
          inputTokens: 100,
          outputTokens: 50,
          cacheCreationTokens: 0,
          cacheReadTokens: 0,
          totalTokens: 150,
          cost: testMoney(0.5),
          durationSeconds: 4,
        },
      },
    ]);
    await tick();
    await tick();

    expect(document.querySelector(".conversation-label")?.textContent?.trim()).toBe(
      "Root conversation",
    );
    const promptLabels = [...document.querySelectorAll(".conversation-label")].map((el) =>
      el.textContent?.trim(),
    );
    expect(promptLabels).toContain("Turn 1");
    expect(document.querySelector(".tag-prompt")?.textContent?.trim()).toBe("Prompt");
    const costs = [...document.querySelectorAll(".row-cost")].map((el) => el.textContent?.trim());
    expect(costs).toContain("$0.50");
    const tokens = [...document.querySelectorAll(".row-tokens")].map((el) => el.textContent?.trim());
    expect(tokens).toContain("150");
    const breakdowns = [...document.querySelectorAll(".token-breakdown")].map((el) =>
      el.textContent?.trim(),
    );
    expect(breakdowns).toContain("in 100 · out 50 · cache write 0 · cache read 0");
  });

  it("shows a Prompt's own text as its label, with the turn number moved into the meta line", async () => {
    usage.topSessions = [
      {
        sessionId: "root-1",
        displayName: "Root conversation",
        agent: "claude",
        project: "demo",
        startedAt: "2026-07-01T00:00:00Z",
        inputTokens: 0,
        outputTokens: 0,
        cacheCreationTokens: 0,
        cacheReadTokens: 0,
        totalTokens: 0,
        cost: testMoney(0),
      },
    ];
    getSessionPrompts.mockResolvedValue([
      {
        id: "root-1:0",
        sessionId: "root-1",
        ordinal: 0,
        startedAt: "2026-07-01T00:00:01Z",
        messageCount: 3,
        text: "What does this function do?",
        usage: {
          inputTokens: 0,
          outputTokens: 0,
          cacheCreationTokens: 0,
          cacheReadTokens: 0,
          totalTokens: 0,
          cost: testMoney(0),
          durationSeconds: 0,
        },
      },
    ]);

    component = mount(ConversationTree, { target: document.body });
    await tick();

    document
      .querySelector<HTMLElement>(".chevron-inline")
      ?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await tick();
    await tick();

    const labels = [...document.querySelectorAll(".conversation-label")].map((el) =>
      el.textContent?.trim(),
    );
    expect(labels).toContain("What does this function do?");
    expect(labels).not.toContain("Turn 1");
    const metas = [...document.querySelectorAll(".conversation-meta")].map((el) =>
      el.textContent?.trim(),
    );
    expect(metas).toContain("Turn 1 · 3 messages");
  });

  it("nests a dispatched sub-agent under its Prompt, tagged with its role", async () => {
    usage.topSessions = [
      {
        sessionId: "root-1",
        displayName: "Root conversation",
        agent: "claude",
        project: "demo",
        startedAt: "2026-07-01T00:00:00Z",
        inputTokens: 0,
        outputTokens: 0,
        cacheCreationTokens: 0,
        cacheReadTokens: 0,
        totalTokens: 0,
        cost: testMoney(0),
      },
    ];
    const zeroUsage = {
      inputTokens: 0,
      outputTokens: 0,
      cacheCreationTokens: 0,
      cacheReadTokens: 0,
      totalTokens: 0,
      cost: testMoney(0),
      durationSeconds: 0,
    };
    getSessionPrompts.mockResolvedValue([
      {
        id: "root-1:0",
        sessionId: "root-1",
        ordinal: 0,
        startedAt: "2026-07-01T00:00:01Z",
        messageCount: 2,
        usage: zeroUsage,
        subSessions: [
          {
            sessionId: "sub-1",
            role: "Explore",
            displayName: "explore the repo",
            agent: "claude",
            startedAt: "2026-07-01T00:00:02Z",
            relationshipType: "subagent",
            usage: zeroUsage,
          },
        ],
      },
    ]);

    component = mount(ConversationTree, { target: document.body });
    await tick();

    document
      .querySelector<HTMLElement>(".chevron-inline")
      ?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await tick();
    await tick();

    // Expand the now-visible Prompt row to reveal the dispatched sub-agent.
    const chevrons = document.querySelectorAll<HTMLElement>(".chevron-inline");
    expect(chevrons).toHaveLength(2);
    chevrons[1]?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await tick();

    const labels = [...document.querySelectorAll(".conversation-label")].map((el) =>
      el.textContent?.trim(),
    );
    expect(labels).toContain("explore the repo");
    const tags = [...document.querySelectorAll(".tag-subagent")].map((el) => el.textContent?.trim());
    expect(tags).toContain("Sub-agent");
    const roleTags = [...document.querySelectorAll(".tag-role")].map((el) => el.textContent?.trim());
    expect(roleTags).toContain("Explore");
  });

  it("tags the first Prompt of a stitched continuation session", async () => {
    usage.topSessions = [
      {
        sessionId: "root-1",
        displayName: "Root conversation",
        agent: "claude",
        project: "demo",
        startedAt: "2026-07-01T00:00:00Z",
        inputTokens: 0,
        outputTokens: 0,
        cacheCreationTokens: 0,
        cacheReadTokens: 0,
        totalTokens: 0,
        cost: testMoney(0),
      },
    ];
    const zeroUsage = {
      inputTokens: 0,
      outputTokens: 0,
      cacheCreationTokens: 0,
      cacheReadTokens: 0,
      totalTokens: 0,
      cost: testMoney(0),
      durationSeconds: 0,
    };
    getSessionPrompts.mockResolvedValue([
      {
        id: "root-1:0",
        sessionId: "root-1",
        ordinal: 0,
        startedAt: "2026-07-01T00:00:01Z",
        messageCount: 1,
        usage: zeroUsage,
      },
      {
        id: "root-1-cont:0",
        sessionId: "root-1-cont",
        ordinal: 0,
        startedAt: "2026-07-01T01:00:00Z",
        messageCount: 1,
        usage: zeroUsage,
        isContinuationStart: true,
      },
    ]);

    component = mount(ConversationTree, { target: document.body });
    await tick();

    document
      .querySelector<HTMLElement>(".chevron-inline")
      ?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await tick();
    await tick();

    const continuationTags = [...document.querySelectorAll(".tag-continuation")].map((el) =>
      el.textContent?.trim(),
    );
    expect(continuationTags).toEqual(["Continuation"]);
  });

  it("shows an error state per session and retries only that session's fetch", async () => {
    usage.topSessions = [
      {
        sessionId: "root-1",
        displayName: "Root conversation",
        agent: "claude",
        project: "demo",
        startedAt: "2026-07-01T00:00:00Z",
        inputTokens: 0,
        outputTokens: 0,
        cacheCreationTokens: 0,
        cacheReadTokens: 0,
        totalTokens: 0,
        cost: testMoney(0),
      },
    ];
    getSessionPrompts.mockRejectedValueOnce(new Error("boom"));

    component = mount(ConversationTree, { target: document.body });
    await tick();

    document
      .querySelector<HTMLElement>(".chevron-inline")
      ?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await tick();
    await tick();

    expect(document.querySelector(".status-error")?.textContent).toContain("boom");

    getSessionPrompts.mockResolvedValueOnce([]);
    document
      .querySelector<HTMLElement>(".status-error .retry-btn")
      ?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await tick();
    await tick();

    expect(getSessionPrompts).toHaveBeenCalledTimes(2);
    expect(document.querySelector(".status-row")?.textContent?.trim()).toBe("No prompts found");
  });
});
