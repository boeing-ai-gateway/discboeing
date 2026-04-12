import type {
	AppPreferences,
	AppSessions,
	AppUI,
	SettingsDialogTab,
} from "$lib/app/app-context.types";

const RECENT_SESSIONS_LIMIT = 4;

export type AppViewState = AppUI & {
	credentialsDialogOpen: boolean;
	openSettingsDialogAt: (tab: SettingsDialogTab) => void;
	openCredentialsDialog: (credentialId?: string | Event) => void;
	closeCredentialsDialog: () => void;
};

function getDefaultSettingsTab(): SettingsDialogTab {
	return "appearance";
}

function compareIsoDatesDesc(left: string, right: string): number {
	const leftTime = new Date(left).getTime();
	const rightTime = new Date(right).getTime();
	if (Number.isNaN(leftTime) || Number.isNaN(rightTime)) {
		return 0;
	}
	return rightTime - leftTime;
}

function getVisibleRecentThreads(args: {
	recentThreads: AppSessions["recentThreads"];
	sessions: AppSessions["sessions"];
	limit: number;
}): AppSessions["recentThreads"] {
	const { recentThreads, sessions, limit } = args;
	if (limit <= 0 || recentThreads.length === 0) {
		return [];
	}

	const sessionsById = new Map(
		sessions.map((session) => [session.id, session]),
	);

	// First pick the most recently visited threads, then keep the sidebar grouped
	// by newer sessions so the list feels stable next to the full session list.
	return [...recentThreads]
		.sort((left, right) =>
			compareIsoDatesDesc(left.lastAccessedAt, right.lastAccessedAt),
		)
		.slice(0, limit)
		.sort((left, right) =>
			compareIsoDatesDesc(
				sessionsById.get(left.sessionId)?.createdAt ?? "",
				sessionsById.get(right.sessionId)?.createdAt ?? "",
			),
		);
}

function getMountedSessionIds(args: {
	selectedSessionId: string | null;
	recentThreads: AppUI["visibleRecentThreads"];
	limit?: number;
}): string[] {
	const {
		selectedSessionId,
		recentThreads,
		limit = RECENT_SESSIONS_LIMIT,
	} = args;
	const sessionIds: string[] = [];
	const seen: Record<string, true> = {};

	// Keep the selected session mounted first, then add sessions referenced by the
	// visible recent-thread list until we hit the small preload budget.
	for (const sessionId of [
		selectedSessionId,
		...recentThreads.map((thread) => thread.sessionId),
	]) {
		if (!sessionId || seen[sessionId]) {
			continue;
		}
		seen[sessionId] = true;
		sessionIds.push(sessionId);
		if (sessionIds.length >= limit) {
			break;
		}
	}

	return sessionIds;
}

export function createAppViewState(args: {
	sessions: AppSessions;
	preferences: AppPreferences;
}): AppViewState {
	const { sessions, preferences } = args;
	let settingsDialogTab = $state<SettingsDialogTab>(getDefaultSettingsTab());
	let credentialFlowIntent = $state<"github-git" | null>(null);
	let credentialsDialogTargetId = $state<string | null>(null);
	let settingsDialogOpen = $state(false);
	let credentialsDialogOpen = $state(false);
	let supportInfoDialogOpen = $state(false);
	const visibleRecentThreads = $derived.by(() =>
		getVisibleRecentThreads({
			recentThreads: sessions.recentThreads,
			sessions: sessions.sessions,
			limit: preferences.recentThreadsVisibleLimit,
		}),
	);
	const mountedSessionIds = $derived.by(() =>
		getMountedSessionIds({
			selectedSessionId: sessions.selectedId,
			recentThreads: visibleRecentThreads,
			limit: RECENT_SESSIONS_LIMIT,
		}),
	);

	const openSettingsDialogAt = (tab: SettingsDialogTab) => {
		credentialsDialogOpen = false;
		supportInfoDialogOpen = false;
		settingsDialogTab = tab;
		settingsDialogOpen = true;
	};

	const settingsDialog = {
		get open() {
			return settingsDialogOpen;
		},
		set open(value: boolean) {
			settingsDialogOpen = value;
		},
		get tab() {
			return settingsDialogTab;
		},
		set tab(value: SettingsDialogTab) {
			settingsDialogTab = value;
		},
	};

	return {
		get credentialFlowIntent() {
			return credentialFlowIntent;
		},
		set credentialFlowIntent(value) {
			credentialFlowIntent = value;
		},
		get credentialsDialogTargetId() {
			return credentialsDialogTargetId;
		},
		set credentialsDialogTargetId(value) {
			credentialsDialogTargetId = value;
		},
		get supportInfoDialogOpen() {
			return supportInfoDialogOpen;
		},
		set supportInfoDialogOpen(value) {
			supportInfoDialogOpen = value;
		},
		get credentialsDialogOpen() {
			return credentialsDialogOpen;
		},
		set credentialsDialogOpen(value) {
			credentialsDialogOpen = value;
		},
		get visibleRecentThreads() {
			return visibleRecentThreads;
		},
		get mountedSessionIds() {
			return mountedSessionIds;
		},
		settingsDialog,
		openSettings: (tab?: SettingsDialogTab) => {
			credentialFlowIntent = null;
			credentialsDialogTargetId = null;
			openSettingsDialogAt(tab ?? getDefaultSettingsTab());
		},
		closeSettings: () => {
			settingsDialogOpen = false;
			settingsDialogTab = getDefaultSettingsTab();
			credentialFlowIntent = null;
			credentialsDialogTargetId = null;
		},
		openGitHubCredentialFlow: () => {
			credentialFlowIntent = "github-git";
			credentialsDialogTargetId = null;
			openSettingsDialogAt("credentials");
		},
		openSupportInfo: () => {
			settingsDialogOpen = false;
			credentialsDialogOpen = false;
			credentialsDialogTargetId = null;
			supportInfoDialogOpen = true;
		},
		closeSupportInfo: () => {
			supportInfoDialogOpen = false;
		},
		openSettingsDialogAt,
		openCredentialsDialog: (credentialId?: string | Event) => {
			credentialsDialogTargetId =
				typeof credentialId === "string" ? credentialId : null;
			credentialsDialogOpen = true;
			openSettingsDialogAt("credentials");
		},
		closeCredentialsDialog: () => {
			credentialsDialogOpen = false;
			credentialsDialogTargetId = null;
		},
	};
}
