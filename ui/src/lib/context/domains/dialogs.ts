import type { Context, SettingsDialogTab } from "$lib/context/context.types";
import type { SwitcherCommitModifier } from "$lib/shortcuts/global-shortcuts";

export async function setSettingsDialogOpen(
	context: Context,
	open: boolean,
): Promise<void> {
	context.view.app.dialogs.settings.open = open;
}

export async function setSettingsDialogTab(
	context: Context,
	tab: SettingsDialogTab,
): Promise<void> {
	context.view.app.dialogs.settings.tab = tab;
}

export async function openSettingsDialog(
	context: Context,
	tab: SettingsDialogTab = "appearance",
): Promise<void> {
	context.view.app.dialogs.settings.open = true;
	context.view.app.dialogs.settings.tab = tab;
}

export async function closeSettingsDialog(context: Context): Promise<void> {
	context.view.app.dialogs.settings.open = false;
}

export async function openCredentialsDialog(
	context: Context,
	credentialId?: string,
): Promise<void> {
	context.view.app.dialogs.credentials.open = true;
	context.view.app.dialogs.credentials.targetId = credentialId ?? null;
}

export async function clearCredentialsDialogTarget(
	context: Context,
): Promise<void> {
	context.view.app.dialogs.credentials.targetId = null;
}

export async function openGitHubCredentialFlow(
	context: Context,
): Promise<void> {
	context.view.app.dialogs.credentials.open = true;
	context.view.app.dialogs.credentials.flowIntent = "github-git";
}

export async function clearCredentialFlowIntent(
	context: Context,
): Promise<void> {
	context.view.app.dialogs.credentials.flowIntent = null;
}

export async function openSupportInfoDialog(context: Context): Promise<void> {
	context.view.app.dialogs.supportInfo.open = true;
}

export async function closeSupportInfoDialog(context: Context): Promise<void> {
	context.view.app.dialogs.supportInfo.open = false;
}

export async function setKeyboardShortcutsOpen(
	context: Context,
	open: boolean,
): Promise<void> {
	context.view.app.dialogs.keyboardShortcuts.open = open;
}

export async function toggleKeyboardShortcutsOpen(
	context: Context,
): Promise<void> {
	context.view.app.dialogs.keyboardShortcuts.open =
		!context.view.app.dialogs.keyboardShortcuts.open;
}

export async function setRecentThreadSwitcherOpen(
	context: Context,
	open: boolean,
): Promise<void> {
	context.view.app.dialogs.recentThreadSwitcher.open = open;
}

export async function setRecentThreadSwitcherSelectedKey(
	context: Context,
	key: string | null,
): Promise<void> {
	context.view.app.dialogs.recentThreadSwitcher.selectedKey = key;
}

export async function setRecentThreadSwitcherCommitModifier(
	context: Context,
	modifier: SwitcherCommitModifier | null,
): Promise<void> {
	context.view.app.dialogs.recentThreadSwitcher.commitModifier = modifier;
}

export async function closeKeyboardShortcutOverlays(
	context: Context,
): Promise<void> {
	context.view.app.dialogs.keyboardShortcuts.open = false;
	context.view.app.dialogs.recentThreadSwitcher.open = false;
	context.view.app.dialogs.recentThreadSwitcher.selectedKey = null;
	context.view.app.dialogs.recentThreadSwitcher.commitModifier = null;
}
