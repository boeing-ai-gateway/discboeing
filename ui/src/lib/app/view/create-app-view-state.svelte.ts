import type {
	AppPreferences,
	AppSessions,
	AppUI,
	SettingsDialogTab,
} from "$lib/app/app-context.types";
import {
	getDefaultSettingsTab,
	getMountedSessionIds,
	getVisibleRecentThreads,
	RECENT_SESSIONS_LIMIT,
} from "$lib/app/app-helpers";

export type AppViewState = AppUI & {
	credentialsDialogOpen: boolean;
	openSettingsDialogAt: (tab: SettingsDialogTab) => void;
	openCredentialsDialog: (credentialId?: string | Event) => void;
	closeCredentialsDialog: () => void;
};

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
		getVisibleRecentThreads(
			sessions.recentThreads,
			preferences.recentThreadsVisibleLimit,
		),
	);
	const mountedSessionIds = $derived.by(() =>
		getMountedSessionIds(
			sessions.selectedId,
			visibleRecentThreads,
			RECENT_SESSIONS_LIMIT,
		),
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
