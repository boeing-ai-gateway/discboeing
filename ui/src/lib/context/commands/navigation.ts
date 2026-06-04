import { getCommandContext } from "$lib/context/commands";
import { runtime } from "$lib/app/app-runtime.svelte";
import {
	DESKTOP_SERVICE_ID,
	VSCODE_SERVICE_ID,
} from "$lib/session/service-ids";

export function setDesktopSidebarOpen(open: boolean): void {
	getCommandContext().view.app.navigation.desktopSidebarOpen = open;
}

export function setMobileSidebarOpen(open: boolean): void {
	getCommandContext().view.app.navigation.mobileSidebarOpen = open;
}

export function toggleDesktopSidebarOpen(): void {
	const context = getCommandContext();
	context.view.app.navigation.desktopSidebarOpen =
		!context.view.app.navigation.desktopSidebarOpen;
}

export function toggleMobileSidebarOpen(): void {
	const context = getCommandContext();
	context.view.app.navigation.mobileSidebarOpen =
		!context.view.app.navigation.mobileSidebarOpen;
}

export function toggleSelectedSessionView(
	viewKind:
		| "terminal"
		| "desktop"
		| "vscode"
		| "file"
		| "diff-review"
		| "services",
): void {
	const context = getCommandContext();
	const sessionId = context.view.app.selection.sessionId;
	if (!sessionId) {
		return;
	}
	const sessionContext = runtime.sessionContexts.get(sessionId);
	if (!sessionContext) {
		return;
	}

	const sessionView = sessionContext.ui;
	if (sessionView.activeView.kind === viewKind) {
		sessionView.openChat();
		return;
	}

	if (viewKind === "terminal") {
		sessionView.openTerminal();
		return;
	}
	if (viewKind === "desktop") {
		sessionView.openDesktop();
		return;
	}
	if (viewKind === "vscode") {
		if (
			sessionContext.services.list.some(
				(service) => service.id === VSCODE_SERVICE_ID,
			)
		) {
			sessionView.openVSCode();
		}
		return;
	}
	if (viewKind === "file") {
		void sessionContext.files.open();
		return;
	}
	if (viewKind === "diff-review") {
		sessionView.openDiffReview();
		return;
	}

	const sessionServices = sessionContext.services.list.filter(
		(service) =>
			service.id !== DESKTOP_SERVICE_ID && service.id !== VSCODE_SERVICE_ID,
	);
	if (sessionServices.length > 0) {
		sessionView.openServices();
	}
}
