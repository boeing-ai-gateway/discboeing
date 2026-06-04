import type { SwitcherCommitModifier } from "$lib/app/global-shortcuts";
import type { SettingsDialogTab } from "$lib/app/app-context.types";
import { getCommandContext } from "$lib/context/commands";

export function closeSettingsDialog(): void {
	const context = getCommandContext();
	context.view.app.dialogs.settings.open = false;
}

export function setSettingsDialogOpen(open: boolean): void {
	const context = getCommandContext();
	context.view.app.dialogs.settings.open = open;
}

export function setSettingsDialogTab(tab: SettingsDialogTab): void {
	const context = getCommandContext();
	context.view.app.dialogs.settings.tab = tab;
}

export function openSettingsDialog(tab?: SettingsDialogTab): void {
	const context = getCommandContext();
	context.view.app.dialogs.settings.open = true;
	if (tab) {
		context.view.app.dialogs.settings.tab = tab;
	}
}

export function openGitHubCredentialFlow(): void {
	const context = getCommandContext();
	context.view.app.dialogs.credentials.flowIntent = "github-git";
	context.view.app.dialogs.credentials.open = true;
}

export function clearCredentialFlowIntent(): void {
	getCommandContext().view.app.dialogs.credentials.flowIntent = null;
}

export function openCredentialsDialog(credentialId?: string): void {
	const context = getCommandContext();
	context.view.app.dialogs.credentials.open = true;
	context.view.app.dialogs.credentials.targetId = credentialId ?? null;
}

export function closeCredentialsDialog(): void {
	const context = getCommandContext();
	context.view.app.dialogs.credentials.open = false;
	context.view.app.dialogs.credentials.targetId = null;
}

export function clearCredentialsDialogTarget(): void {
	getCommandContext().view.app.dialogs.credentials.targetId = null;
}

export function closeSupportInfoDialog(): void {
	getCommandContext().view.app.dialogs.supportInfo.open = false;
}

export function openSupportInfoDialog(): void {
	getCommandContext().view.app.dialogs.supportInfo.open = true;
}

export function setKeyboardShortcutsOpen(open: boolean): void {
	getCommandContext().view.app.dialogs.keyboardShortcuts.open = open;
}

export function toggleKeyboardShortcutsOpen(): void {
	const context = getCommandContext();
	context.view.app.dialogs.keyboardShortcuts.open =
		!context.view.app.dialogs.keyboardShortcuts.open;
}

export function setRecentThreadSwitcherOpen(open: boolean): void {
	getCommandContext().view.app.dialogs.recentThreadSwitcher.open = open;
}

export function setRecentThreadSwitcherSelectedKey(
	selectedKey: string | null,
): void {
	getCommandContext().view.app.dialogs.recentThreadSwitcher.selectedKey =
		selectedKey;
}

export function setRecentThreadSwitcherCommitModifier(
	commitModifier: SwitcherCommitModifier | null,
): void {
	getCommandContext().view.app.dialogs.recentThreadSwitcher.commitModifier =
		commitModifier;
}

export function closeKeyboardShortcutOverlays(): void {
	const context = getCommandContext();
	context.view.app.dialogs.keyboardShortcuts.open = false;
	context.view.app.dialogs.recentThreadSwitcher.open = false;
	context.view.app.dialogs.recentThreadSwitcher.selectedKey = null;
	context.view.app.dialogs.recentThreadSwitcher.commitModifier = null;
}
