import { beforeEach, describe, expect, test, vi } from "vitest";

import { createCommands } from "$lib/context/commands";
import type { Context } from "$lib/context/context.types";
import { ensureSessionView, ensureThreadView } from "$lib/context/domains/view";
import {
	createInitialDataState,
	createInitialViewState,
} from "$lib/context/initial-state";

const apiMock = vi.hoisted(() => ({
	deleteSessionFile: vi.fn(),
	getSessionDiff: vi.fn(),
	renameSessionFile: vi.fn(),
	writeSessionFile: vi.fn(),
}));

vi.mock("$lib/api-client", async (importOriginal) => {
	const original = await importOriginal<typeof import("$lib/api-client")>();
	return {
		...original,
		api: apiMock,
	};
});

describe("ng view commands", () => {
	beforeEach(() => {
		apiMock.deleteSessionFile.mockReset();
		apiMock.getSessionDiff.mockReset();
		apiMock.renameSessionFile.mockReset();
		apiMock.writeSessionFile.mockReset();
	});

	test("updates navigation and selection state", async () => {
		const context = createPlainContext();

		await context.commands.navigation.setDesktopSidebarOpen(true);
		await context.commands.navigation.toggleMobileSidebarOpen();
		await context.commands.navigation.selectSession("session-1");
		await context.commands.navigation.openThread("session-1", "thread-1");

		expect(context.view.navigation.desktopSidebarOpen).toBe(true);
		expect(context.view.navigation.mobileSidebarOpen).toBe(true);
		expect(context.view.selection.sessionId).toBe("session-1");
		expect(context.view.selection.threadId).toBe("thread-1");
		expect(context.view.selection.requestedThreadIdBySessionId).toEqual({
			"session-1": "thread-1",
		});
		expect(context.view.navigation.mountedSessionIds).toContain("session-1");
		expect(
			context.view.sessions["session-1"]?.threads["thread-1"],
		).toMatchObject({
			sessionId: "session-1",
			threadId: "thread-1",
		});
	});

	test("updates dialog state", async () => {
		const context = createPlainContext();

		await context.commands.dialogs.openSettingsDialog("credentials");
		await context.commands.dialogs.openCredentialsDialog("credential-1");
		await context.commands.dialogs.openGitHubCredentialFlow();
		await context.commands.dialogs.openSupportInfoDialog();
		await context.commands.dialogs.setKeyboardShortcutsOpen(true);
		await context.commands.dialogs.setRecentThreadSwitcherOpen(true);
		await context.commands.dialogs.setRecentThreadSwitcherSelectedKey(
			"session-1:thread-1",
		);
		await context.commands.dialogs.setRecentThreadSwitcherCommitModifier(
			"Control",
		);
		await context.commands.dialogs.closeKeyboardShortcutOverlays();

		expect(context.view.app.dialogs.settings).toEqual({
			open: true,
			tab: "credentials",
		});
		expect(context.view.app.dialogs.credentials).toEqual({
			open: true,
			targetId: "credential-1",
			flowIntent: "github-git",
		});
		expect(context.view.app.dialogs.supportInfo.open).toBe(true);
		expect(context.view.app.dialogs.keyboardShortcuts.open).toBe(false);
		expect(context.view.app.dialogs.recentThreadSwitcher).toEqual({
			open: false,
			selectedKey: null,
			commitModifier: null,
		});
	});

	test("updates preference state", async () => {
		const context = createPlainContext();

		await context.commands.preferences.setPreferredIde("vscode");
		await context.commands.preferences.setChatWidthMode("full");
		await context.commands.preferences.setAutoScrollOnStream(false);
		await context.commands.preferences.setSidebarRecentOpen(false);
		await context.commands.preferences.setSidebarAllOpen(false);
		await context.commands.preferences.setSidebarAllGroupedByWorkspace(false);

		expect(context.view.app.preferences.preferredIde).toBe("vscode");
		expect(context.view.app.preferences.chatWidthMode).toBe("full");
		expect(context.view.app.preferences.autoScrollOnStream).toBe(false);
		expect(context.view.app.preferences.sidebarRecentOpen).toBe(false);
		expect(context.view.app.preferences.sidebarAllOpen).toBe(false);
		expect(context.view.app.preferences.sidebarAllGroupedByWorkspace).toBe(
			false,
		);
	});

	test("starts a new pending session", async () => {
		const context = createPlainContext();
		const previousPendingSessionId = context.view.selection.pendingSessionId;

		await context.commands.navigation.selectSession("session-1");
		await context.commands.navigation.openThread("session-1", "thread-1");
		await context.commands.navigation.startNewSession();

		expect(context.view.selection.sessionId).toBeNull();
		expect(context.view.selection.threadId).toBeNull();
		expect(context.view.selection.pendingSessionId).toBeTruthy();
		expect(context.view.selection.pendingSessionId).not.toBe(
			previousPendingSessionId,
		);
		expect(context.view.navigation.mountedSessionIds).toContain(
			context.view.selection.pendingSessionId,
		);
	});

	test("completes a pending session with a fresh pending placeholder", async () => {
		const context = createPlainContext();

		await context.commands.navigation.startNewSession();
		const pendingSessionId = context.view.selection.pendingSessionId;
		await context.commands.navigation.completePendingSession(
			pendingSessionId,
			"session-1",
		);

		expect(context.view.selection.sessionId).toBe("session-1");
		expect(context.view.selection.threadId).toBe("session-1");
		expect(context.view.selection.requestedThreadIdBySessionId).toEqual({
			"session-1": "session-1",
		});
		expect(context.view.selection.pendingSessionId).toBeTruthy();
		expect(context.view.selection.pendingSessionId).not.toBe(pendingSessionId);
		expect(context.view.navigation.mountedSessionIds).toContain("session-1");
	});

	test("selects the primary thread when selecting a session", async () => {
		const context = createPlainContext();

		await context.commands.navigation.selectSession("session-1");

		expect(context.view.selection.sessionId).toBe("session-1");
		expect(context.view.selection.threadId).toBe("session-1");
		expect(context.view.selection.requestedThreadIdBySessionId).toEqual({
			"session-1": "session-1",
		});
		expect(
			context.view.sessions["session-1"]?.threads["session-1"],
		).toMatchObject({
			sessionId: "session-1",
			threadId: "session-1",
		});
	});

	test("toggles the selected session workspace view", async () => {
		const context = createPlainContext();

		await context.commands.navigation.selectSession("session-1");
		await context.commands.navigation.toggleSelectedSessionView("terminal");

		expect(context.view.sessions["session-1"]?.workspace.activeView).toBe(
			"terminal",
		);

		await context.commands.navigation.toggleSelectedSessionView("terminal");

		expect(context.view.sessions["session-1"]?.workspace.activeView).toBe(
			"chat",
		);
	});

	test("opens the session files panel", async () => {
		const context = createPlainContext();
		const session = ensureSessionView(context, "session-1");
		session.files.selected = "src/main.ts";
		session.files.activePath = "src/main.ts";

		await context.commands.files.openFilesPanel("session-1");

		expect(session.workspace.activeView).toBe("file");
		expect(session.files.selected).toBe("");
		expect(session.files.activePath).toBe("");
	});

	test("sets the session diff target and refreshes its summary", async () => {
		const context = createPlainContext();
		apiMock.getSessionDiff.mockResolvedValueOnce({
			files: [{ path: "src/target-only.ts", status: "modified" }],
			stats: { filesChanged: 1, additions: 2, deletions: 1 },
		});

		await context.commands.files.setDiffTarget("session-1", " HEAD ");

		expect(apiMock.getSessionDiff).toHaveBeenCalledWith("session-1", {
			format: "files",
			target: "HEAD",
		});
		expect(context.view.sessions["session-1"]?.files.diffTarget).toBe("HEAD");
		expect(
			context.view.sessions["session-1"]?.files.diffFilesByTarget["HEAD"],
		).toEqual({
			files: [{ path: "src/target-only.ts", status: "modified" }],
			stats: { filesChanged: 1, additions: 2, deletions: 1 },
		});
	});

	test("refreshes the default merge-base diff summary", async () => {
		const context = createPlainContext();
		apiMock.getSessionDiff.mockResolvedValueOnce({
			files: [{ path: "src/default.ts", status: "modified" }],
			stats: { filesChanged: 1, additions: 3, deletions: 1 },
		});

		await context.commands.files.setDiffTarget("session-1", " ");

		expect(apiMock.getSessionDiff).toHaveBeenCalledWith("session-1", {
			format: "files",
			target: "",
		});
		expect(context.view.sessions["session-1"]?.files.diffTarget).toBe("");
		expect(context.data.sessions.byId["session-1"]?.diff.files).toEqual({
			files: [{ path: "src/default.ts", status: "modified" }],
			stats: { filesChanged: 1, additions: 3, deletions: 1 },
		});
	});

	test("keeps default diff summary reference when refresh is unchanged", async () => {
		const context = createPlainContext();
		const diff = {
			files: [{ path: "src/default.ts", status: "modified" }],
			stats: { filesChanged: 1, additions: 3, deletions: 1 },
		};
		apiMock.getSessionDiff.mockResolvedValueOnce(diff);
		await context.commands.files.setDiffTarget("session-1", "");
		const existingDiff = context.data.sessions.byId["session-1"]?.diff.files;

		apiMock.getSessionDiff.mockResolvedValueOnce(structuredClone(diff));
		await context.commands.files.setDiffTarget("session-1", "");

		expect(apiMock.getSessionDiff).toHaveBeenCalledTimes(2);
		expect(context.data.sessions.byId["session-1"]?.diff.files).toBe(
			existingDiff,
		);
	});

	test("refreshes an already cached session diff target summary", async () => {
		const context = createPlainContext();
		apiMock.getSessionDiff
			.mockResolvedValueOnce({
				files: [{ path: "src/stale.ts", status: "modified" }],
				stats: { filesChanged: 1, additions: 1, deletions: 0 },
			})
			.mockResolvedValueOnce({
				files: [{ path: "src/fresh.ts", status: "added" }],
				stats: { filesChanged: 1, additions: 5, deletions: 0 },
			});

		await context.commands.files.setDiffTarget("session-1", "HEAD");
		await context.commands.files.setDiffTarget("session-1", "HEAD");

		expect(apiMock.getSessionDiff).toHaveBeenCalledTimes(2);
		expect(
			context.view.sessions["session-1"]?.files.diffFilesByTarget["HEAD"],
		).toEqual({
			files: [{ path: "src/fresh.ts", status: "added" }],
			stats: { filesChanged: 1, additions: 5, deletions: 0 },
		});
	});

	test("invalidates cached diff target summaries after file mutations", async () => {
		const context = createPlainContext();
		const session = ensureSessionView(context, "session-1");
		session.files.diffFilesByTarget.HEAD = {
			files: [{ path: "src/stale.ts", status: "modified" }],
			stats: { filesChanged: 1, additions: 1, deletions: 0 },
		};
		apiMock.writeSessionFile.mockResolvedValueOnce(undefined);

		await context.commands.files.saveFile("session-1", "src/stale.ts", "next");

		expect(apiMock.writeSessionFile).toHaveBeenCalledWith("session-1", {
			path: "src/stale.ts",
			content: "next",
		});
		expect(session.files.diffFilesByTarget).toEqual({});
	});

	test("saves files with conflict metadata and invalidates target summaries", async () => {
		const context = createPlainContext();
		const session = ensureSessionView(context, "session-1");
		session.files.diffFilesByTarget.HEAD = {
			files: [{ path: "src/stale.ts", status: "modified" }],
			stats: { filesChanged: 1, additions: 1, deletions: 0 },
		};
		apiMock.writeSessionFile.mockResolvedValueOnce(undefined);

		await context.commands.files.saveFile("session-1", "src/stale.ts", "next", {
			encoding: "utf8",
			originalContent: "previous",
		});

		expect(apiMock.writeSessionFile).toHaveBeenCalledWith("session-1", {
			path: "src/stale.ts",
			content: "next",
			encoding: "utf8",
			originalContent: "previous",
		});
		expect(session.files.diffFilesByTarget).toEqual({});
	});

	test("updates composer-owned session view state through view commands", async () => {
		const context = createPlainContext();
		const session = ensureSessionView(context, "session-1");
		session.pendingWorkspace.option = "local";
		session.pendingWorkspace.branch = "feature";
		session.pendingWorkspace.sourceInput = "/tmp/project";
		session.pendingWorkspace.validation = {
			path: "/tmp/project",
			sourceType: "local",
			valid: false,
			classification: "invalid",
			error: "bad path",
			suggestions: [],
		};
		session.pendingWorkspace.validating = true;
		session.pendingWorkspace.setupMessage = "Preparing workspace";

		await context.commands.view.setSessionHooksExpanded("session-1", true);
		await context.commands.view.setPendingWorkspaceSandboxProviderId(
			"session-1",
			"provider-1",
		);

		expect(session.hooks.expanded).toBe(true);
		expect(session.pendingWorkspace.sandboxProviderId).toBe("provider-1");

		await context.commands.view.resetPendingWorkspaceSetup("session-1");

		expect(session.pendingWorkspace).toMatchObject({
			option: "",
			branch: "",
			sourceInput: "",
			validation: null,
			validating: false,
			setupMessage: null,
			sandboxProviderId: "",
		});
	});

	test("opens the session service panel", async () => {
		const context = createPlainContext();

		await context.commands.services.openServicePanel(
			"session-1",
			"service-1",
			"logs",
		);

		expect(context.view.sessions["session-1"]?.workspace.activeView).toBe(
			"services",
		);
		expect(context.view.sessions["session-1"]?.workspace.activeServiceId).toBe(
			"service-1",
		);
		expect(context.view.sessions["session-1"]?.services.activeServiceId).toBe(
			"service-1",
		);
		expect(context.view.sessions["session-1"]?.services.activeViewMode).toBe(
			"logs",
		);
	});

	test("creates stable session and thread view records", () => {
		const context = createPlainContext();

		const session = ensureSessionView(context, "session-1");
		const sameSession = ensureSessionView(context, "session-1");
		const thread = ensureThreadView(context, "session-1", "thread-1");
		const sameThread = ensureThreadView(context, "session-1", "thread-1");

		expect(sameSession).toBe(session);
		expect(sameThread).toBe(thread);
		expect(session.files.openPaths).toEqual([]);
		expect(thread.composer.pendingComments).toEqual([]);
	});
});

function createPlainContext(): Context {
	const context: Context = {
		data: createInitialDataState({ projectId: "local" }),
		view: createInitialViewState({
			selectedSessionId: "boot-session",
			selectedThreadId: "boot-thread",
		}),
		commands: undefined as unknown as Context["commands"],
	};
	context.commands = createCommands(context);
	return context;
}
