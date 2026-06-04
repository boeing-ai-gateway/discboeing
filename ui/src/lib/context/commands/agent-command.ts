import type { AgentCommand } from "$lib/api-types";
import type {
	CredentialValidityPreset,
	CredentialValidityUnit,
} from "$lib/context/context.types";
import {
	ensureRuntimeSessionState,
	refreshRuntimeCommands,
	runRuntimeCommand,
	syncRuntimeProjections,
} from "$lib/app/app-runtime.svelte";

export async function refreshAgentCommands(sessionId: string): Promise<void> {
	await refreshRuntimeCommands(sessionId);
}

export async function runAgentCommand(
	sessionId: string,
	command: AgentCommand,
): Promise<void> {
	await runRuntimeCommand(sessionId, command);
}

export function closeAgentCommandCredentialDialog(sessionId: string): void {
	ensureRuntimeSessionState(sessionId).commands.credentialDialog.close();
	syncRuntimeProjections();
}

export async function confirmAgentCommandCredentialDialog(
	sessionId: string,
): Promise<void> {
	await ensureRuntimeSessionState(
		sessionId,
	).commands.credentialDialog.confirm();
	syncRuntimeProjections();
}

export function selectAgentCommandCredentialOption(
	sessionId: string,
	envVar: string,
	value: string,
): void {
	ensureRuntimeSessionState(sessionId).commands.credentialDialog.selectOption(
		envVar,
		value,
	);
	syncRuntimeProjections();
}

export function setAgentCommandCredentialCreateName(
	sessionId: string,
	envVar: string,
	value: string,
): void {
	ensureRuntimeSessionState(
		sessionId,
	).commands.credentialDialog.setCreateCredentialName(envVar, value);
	syncRuntimeProjections();
}

export function setAgentCommandCredentialCreateSecret(
	sessionId: string,
	envVar: string,
	value: string,
): void {
	ensureRuntimeSessionState(
		sessionId,
	).commands.credentialDialog.setCreateCredentialSecret(envVar, value);
	syncRuntimeProjections();
}

export function setAgentCommandCredentialValidityPreset(
	sessionId: string,
	envVar: string,
	value: CredentialValidityPreset,
): void {
	ensureRuntimeSessionState(
		sessionId,
	).commands.credentialDialog.setValidityPreset(envVar, value);
	syncRuntimeProjections();
}

export function setAgentCommandCredentialValidityValue(
	sessionId: string,
	envVar: string,
	value: string,
): void {
	ensureRuntimeSessionState(
		sessionId,
	).commands.credentialDialog.setValidityValue(envVar, value);
	syncRuntimeProjections();
}

export function setAgentCommandCredentialValidityUnit(
	sessionId: string,
	envVar: string,
	value: CredentialValidityUnit,
): void {
	ensureRuntimeSessionState(
		sessionId,
	).commands.credentialDialog.setValidityUnit(envVar, value);
	syncRuntimeProjections();
}

export async function launchAgentCommandCredentialOAuthWizard(
	sessionId: string,
	envVar: string,
): Promise<void> {
	await ensureRuntimeSessionState(
		sessionId,
	).commands.credentialDialog.launchOAuthWizard(envVar);
	syncRuntimeProjections();
}

export async function refreshAgentCommandCredentialDialogCredentials(
	sessionId: string,
): Promise<void> {
	await ensureRuntimeSessionState(
		sessionId,
	).commands.credentialDialog.refreshAvailableCredentials();
	syncRuntimeProjections();
}
