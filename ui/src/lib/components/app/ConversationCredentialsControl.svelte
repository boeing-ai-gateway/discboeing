<script lang="ts">
	import FishingHookIcon from "@lucide/svelte/icons/fishing-hook";
	import HammerIcon from "@lucide/svelte/icons/hammer";
	import KeyRoundIcon from "@lucide/svelte/icons/key-round";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import SettingsIcon from "@lucide/svelte/icons/settings";
	import ServerIcon from "@lucide/svelte/icons/server";
	import TerminalIcon from "@lucide/svelte/icons/terminal";
	import Trash2Icon from "@lucide/svelte/icons/trash-2";
	import { api } from "$lib/api-client";
	import type {
		CredentialVisibility,
		SessionCredentialAssignment,
	} from "$lib/api-types";
	import {
		AlertDialog,
		AlertDialogAction,
		AlertDialogCancel,
		AlertDialogContent,
		AlertDialogDescription,
		AlertDialogFooter,
		AlertDialogHeader,
		AlertDialogTitle,
	} from "$lib/components/ui/alert-dialog";
	import { Button } from "$lib/components/ui/button";
	import { Checkbox } from "$lib/components/ui/checkbox";
	import {
		DropdownMenu,
		DropdownMenuContent,
		DropdownMenuLabel,
		DropdownMenuSeparator,
		DropdownMenuTrigger,
	} from "$lib/components/ui/dropdown-menu";
	import { useAppContext } from "$lib/context/app-context.svelte";
	import { useSessionContext } from "$lib/context/session-context.svelte";

	const app = useAppContext();
	const session = useSessionContext();
	const componentId = `session-credentials-${Math.random().toString(36).slice(2)}`;

	let assignments = $state<SessionCredentialAssignment[]>([]);
	let loading = $state(false);
	let loadedSessionId = $state<string | null>(null);
	let globalVisibilityDialogOpen = $state(false);
	let globalVisibilityDialogAssignment =
		$state<SessionCredentialAssignment | null>(null);
	let globalVisibilityDialogContexts = $state<string[]>([]);
	let expandedUses = $state<Record<string, boolean>>({});

	const visibleCount = $derived.by(
		() =>
			assignments.filter(
				(assignment) =>
					hasAnyVisibility(effectiveVisibility(assignment)) &&
					!assignment.credential.inactive,
			).length,
	);

	async function loadAssignments() {
		if (session.isPending) {
			assignments = [];
			return;
		}
		loading = true;
		try {
			const response = await api.getSessionCredentials(session.sessionId);
			assignments = response.credentials;
			await app.credentials.refresh();
		} finally {
			loading = false;
		}
	}

	function notifySessionCredentialsChanged() {
		if (typeof window === "undefined") {
			return;
		}
		window.dispatchEvent(
			new CustomEvent("discobot:session-credentials-changed", {
				detail: { sessionId: session.sessionId, source: componentId },
			}),
		);
	}

	async function saveAssignments(
		nextAssignments: SessionCredentialAssignment[],
	) {
		const response = await api.setSessionCredentials(
			session.sessionId,
			nextAssignments
				.filter(
					(assignment) =>
						Boolean(assignment.sessionCredentialId) ||
						Boolean(assignment.envVar) ||
						Boolean(assignment.sourceEnvVar) ||
						(assignment.uses?.length ?? 0) > 0 ||
						assignment.visibility.tools !==
							assignment.credential.visibility.tools ||
						assignment.visibility.console !==
							assignment.credential.visibility.console ||
						assignment.visibility.services !==
							assignment.credential.visibility.services ||
						assignment.visibility.hooks !==
							assignment.credential.visibility.hooks,
				)
				.map((assignment) => ({
					credentialId: assignment.credentialId,
					sessionCredentialId: assignment.sessionCredentialId,
					envVar: assignment.envVar,
					sourceEnvVar: assignment.sourceEnvVar,
					agentVisible: assignment.visibility.tools,
					visibility: assignment.visibility,
					uses: assignment.uses,
				})),
		);
		assignments = response.credentials;
		notifySessionCredentialsChanged();
	}

	function credentialDisplayName(assignment: SessionCredentialAssignment) {
		const { credential } = assignment;
		const name = credential.name.trim();
		if (name.length > 0) {
			return name;
		}
		if (credential.provider.startsWith("custom:")) {
			return credential.envKeys?.join(", ") || "Custom env vars";
		}
		const matchedType = app.credentials.credentialTypes.find(
			(type) =>
				type.backendProvider === credential.provider &&
				type.authType === credential.authType,
		);
		if (matchedType) {
			return matchedType.name;
		}
		return credential.provider;
	}

	function assignmentKey(assignment: SessionCredentialAssignment) {
		return [
			assignment.credentialId,
			assignment.envVar ?? "",
			assignment.sessionCredentialId ?? "",
		].join("\x00");
	}

	function useIsExpired(
		use: NonNullable<SessionCredentialAssignment["uses"]>[number],
	) {
		if (!use.expiresAt) {
			return false;
		}
		return new Date(use.expiresAt).getTime() <= Date.now();
	}

	function formatDuration(milliseconds: number) {
		if (!Number.isFinite(milliseconds) || milliseconds <= 0) {
			return "0m";
		}
		const totalMinutes = Math.round(milliseconds / 60000);
		if (totalMinutes < 60) {
			return `${totalMinutes}m`;
		}
		const hours = Math.floor(totalMinutes / 60);
		const minutes = totalMinutes % 60;
		if (hours < 24) {
			return minutes === 0 ? `${hours}h` : `${hours}h ${minutes}m`;
		}
		const days = Math.floor(hours / 24);
		const remainingHours = hours % 24;
		return remainingHours === 0 ? `${days}d` : `${days}d ${remainingHours}h`;
	}

	function formatUseTiming(
		use: NonNullable<SessionCredentialAssignment["uses"]>[number],
	) {
		const createdAt = use.createdAt ? new Date(use.createdAt).getTime() : NaN;
		const expiresAt = use.expiresAt ? new Date(use.expiresAt).getTime() : NaN;
		const duration =
			Number.isFinite(createdAt) && Number.isFinite(expiresAt)
				? formatDuration(expiresAt - createdAt)
				: null;
		if (!Number.isFinite(expiresAt)) {
			return duration ? `Valid for ${duration}` : "No expiration";
		}
		const remaining = expiresAt - Date.now();
		if (remaining <= 0) {
			return duration ? `Expired • was valid for ${duration}` : "Expired";
		}
		const remainingText = formatDuration(remaining);
		return duration
			? `Valid for ${duration} • ${remainingText} left`
			: `${remainingText} left`;
	}

	function toggleUses(assignment: SessionCredentialAssignment) {
		const key = assignmentKey(assignment);
		expandedUses = {
			...expandedUses,
			[key]: !expandedUses[key],
		};
	}

	async function removeUse(
		targetAssignment: SessionCredentialAssignment,
		useId: string,
	) {
		const key = assignmentKey(targetAssignment);
		const nextAssignments = assignments.map((assignment) => {
			if (assignmentKey(assignment) !== key) {
				return assignment;
			}
			return {
				...assignment,
				uses: (assignment.uses ?? []).filter((use) => use.id !== useId),
			};
		});
		await saveAssignments(nextAssignments);
	}

	function hasAnyVisibility(visibility: CredentialVisibility) {
		return (
			visibility.tools ||
			visibility.console ||
			visibility.services ||
			visibility.hooks
		);
	}

	function effectiveVisibility(
		assignment: SessionCredentialAssignment,
	): CredentialVisibility {
		if (assignment.credential.inactive) {
			return {
				tools: false,
				console: false,
				services: false,
				hooks: false,
			};
		}
		return {
			tools:
				assignment.credential.visibility.tools || assignment.visibility.tools,
			console:
				assignment.credential.visibility.console ||
				assignment.visibility.console,
			services:
				assignment.credential.visibility.services ||
				assignment.visibility.services,
			hooks:
				assignment.credential.visibility.hooks || assignment.visibility.hooks,
		};
	}

	function allVisibilityCheckedState(visibility: CredentialVisibility) {
		const enabledCount = [
			visibility.tools,
			visibility.console,
			visibility.services,
			visibility.hooks,
		].filter(Boolean).length;
		if (enabledCount === 4) {
			return { checked: true, indeterminate: false };
		}
		if (enabledCount === 0) {
			return { checked: false, indeterminate: false };
		}
		return { checked: false, indeterminate: true };
	}

	function runtimeDisabled(
		assignment: SessionCredentialAssignment,
		_key?: keyof CredentialVisibility,
	) {
		return assignment.credential.inactive;
	}

	function runtimeLockedByGlobal(
		assignment: SessionCredentialAssignment,
		key: keyof CredentialVisibility,
	) {
		return assignment.credential.visibility[key];
	}

	function lockedGlobalContexts(assignment: SessionCredentialAssignment) {
		const contexts: Array<{ key: keyof CredentialVisibility; label: string }> =
			[
				{ key: "tools", label: "Tools" },
				{ key: "console", label: "Console / SSH / IDE" },
				{ key: "services", label: "Services" },
				{ key: "hooks", label: "Hooks" },
			];
		return contexts.filter(
			(context) => assignment.credential.visibility[context.key],
		);
	}

	function openGlobalVisibilityDialog(
		assignment: SessionCredentialAssignment,
		contexts: string[],
	) {
		globalVisibilityDialogAssignment = assignment;
		globalVisibilityDialogContexts = contexts;
		globalVisibilityDialogOpen = true;
	}

	function closeGlobalVisibilityDialog() {
		globalVisibilityDialogOpen = false;
		globalVisibilityDialogAssignment = null;
		globalVisibilityDialogContexts = [];
	}

	function openCredentialForGlobalVisibilityEdit() {
		const credentialId = globalVisibilityDialogAssignment?.credential.id;
		closeGlobalVisibilityDialog();
		if (!credentialId) {
			return;
		}
		app.ui.openCredentialsDialog(credentialId);
	}

	function visibilityToggleClass(enabled: boolean, disabled: boolean) {
		if (disabled) {
			return "border-border/60 bg-muted/40 text-muted-foreground opacity-50";
		}
		if (enabled) {
			return "border-yellow-500/40 bg-yellow-500/12 text-yellow-600 shadow-sm dark:text-yellow-400";
		}
		return "border-transparent bg-muted/55 text-muted-foreground hover:bg-muted";
	}

	async function setVisibility(
		credentialId: string,
		key: keyof CredentialVisibility,
		value: boolean,
	) {
		const nextAssignments = assignments.map((assignment) =>
			assignment.credentialId === credentialId
				? assignment.credential.inactive
					? assignment
					: {
							...assignment,
							agentVisible:
								key === "tools" ? value : assignment.visibility.tools,
							visibility: {
								...assignment.visibility,
								[key]: value,
							},
						}
				: assignment,
		);
		await saveAssignments(nextAssignments);
	}

	function handleRuntimeToggle(
		assignment: SessionCredentialAssignment,
		key: keyof CredentialVisibility,
	) {
		if (assignment.credential.inactive) {
			return;
		}
		if (runtimeLockedByGlobal(assignment, key)) {
			openGlobalVisibilityDialog(assignment, [
				key === "tools"
					? "Tools"
					: key === "console"
						? "Console / SSH / IDE"
						: key === "services"
							? "Services"
							: "Hooks",
			]);
			return;
		}
		void setVisibility(
			assignment.credential.id,
			key,
			!assignment.visibility[key],
		);
	}

	async function setAllVisibility(
		credentialId: string,
		value: boolean,
		preserveGlobalLocks = false,
	) {
		const nextAssignments = assignments.map((assignment) =>
			assignment.credentialId === credentialId
				? assignment.credential.inactive
					? assignment
					: {
							...assignment,
							agentVisible:
								preserveGlobalLocks && assignment.credential.visibility.tools
									? assignment.visibility.tools
									: value,
							visibility: {
								tools:
									preserveGlobalLocks && assignment.credential.visibility.tools
										? assignment.visibility.tools
										: value,
								console:
									preserveGlobalLocks &&
									assignment.credential.visibility.console
										? assignment.visibility.console
										: value,
								services:
									preserveGlobalLocks &&
									assignment.credential.visibility.services
										? assignment.visibility.services
										: value,
								hooks:
									preserveGlobalLocks && assignment.credential.visibility.hooks
										? assignment.visibility.hooks
										: value,
							},
						}
				: assignment,
		);
		await saveAssignments(nextAssignments);
	}

	function handleAllVisibilityToggle(assignment: SessionCredentialAssignment) {
		if (assignment.credential.inactive) {
			return;
		}
		const allState = allVisibilityCheckedState(effectiveVisibility(assignment));
		if (allState.checked) {
			void setAllVisibility(assignment.credential.id, false, true);
			return;
		}
		void setAllVisibility(assignment.credential.id, true);
	}

	$effect(() => {
		const sessionId = session.sessionId;
		const isPending = session.isPending;
		if (isPending) {
			assignments = [];
			loadedSessionId = null;
			return;
		}
		if (loadedSessionId === sessionId) {
			return;
		}
		loadedSessionId = sessionId;
		void loadAssignments();
	});

	$effect(() => {
		if (typeof window === "undefined") {
			return;
		}
		const handleSessionCredentialsChanged = (event: Event) => {
			const detail = (
				event as CustomEvent<{
					sessionId?: string;
					source?: string;
				}>
			).detail;
			if (!detail?.sessionId || detail.sessionId !== session.sessionId) {
				return;
			}
			if (detail.source === componentId) {
				return;
			}
			void loadAssignments();
		};
		const handleCredentialsChanged = () => {
			void loadAssignments();
		};
		window.addEventListener(
			"discobot:session-credentials-changed",
			handleSessionCredentialsChanged,
		);
		window.addEventListener(
			"discobot:credentials-changed",
			handleCredentialsChanged,
		);
		return () => {
			window.removeEventListener(
				"discobot:session-credentials-changed",
				handleSessionCredentialsChanged,
			);
			window.removeEventListener(
				"discobot:credentials-changed",
				handleCredentialsChanged,
			);
		};
	});
</script>

<DropdownMenu>
	<DropdownMenuTrigger class="tauri-no-drag">
		<Button
			variant="ghost"
			size="xs"
			class="h-6 gap-1.5 px-2 text-xs"
			aria-label="Select session credentials"
		>
			<KeyRoundIcon
				class={`size-3.5 ${visibleCount > 0 ? "text-yellow-500" : "text-muted-foreground"}`}
			/>
			<span
				class={`inline-flex min-w-3 items-center justify-center text-[10px] tabular-nums ${
					visibleCount > 1 ? "text-foreground" : "invisible"
				}`}
				aria-hidden={visibleCount <= 1}
			>
				{visibleCount}
			</span>
		</Button>
	</DropdownMenuTrigger>
	<DropdownMenuContent align="start" class="w-[28rem] max-w-[calc(100vw-1rem)]">
		<DropdownMenuLabel
			class="text-xs uppercase tracking-[0.16em] text-muted-foreground"
		>
			Session credentials
		</DropdownMenuLabel>
		<div class="px-2 pb-1 text-xs text-muted-foreground">
			Credential visibility for this session. Toggle which runtimes can use each
			credential.
		</div>
		{#if loading}
			<div class="px-2 py-3 text-sm text-muted-foreground">Loading…</div>
		{:else if assignments.length === 0}
			<div class="px-2 py-3 text-sm text-muted-foreground">No credentials</div>
		{:else}
			<div class="space-y-1.5 p-2">
				{#each assignments as assignment (assignment.credentialId)}
					{@const credential = assignment.credential}
					{@const effective = effectiveVisibility(assignment)}
					{@const allVisibilityState = allVisibilityCheckedState(effective)}
					<div
						class="rounded-md border border-border/70 bg-background/70 px-2.5 py-2"
					>
						<div class="flex items-start gap-2">
							<div class="min-w-0 flex-1">
								<div class="truncate text-sm font-medium">
									{credentialDisplayName(assignment)}
								</div>
								{#if (assignment.uses?.length ?? 0) > 0}
									<div class="mt-1">
										<button
											type="button"
											class="inline-flex items-center gap-1 rounded-sm text-[11px] text-muted-foreground hover:text-foreground"
											onclick={() => toggleUses(assignment)}
										>
											<ChevronDownIcon
												class={`size-3 transition-transform ${
													expandedUses[assignmentKey(assignment)]
														? "rotate-180"
														: ""
												}`}
											/>
											<span>
												{assignment.uses?.length}
												{assignment.uses?.length === 1 ? " use" : " uses"}
											</span>
										</button>
									</div>
								{/if}
							</div>
							<div class="flex items-center gap-1">
								<button
									type="button"
									title="Tools"
									aria-label="Toggle tools visibility"
									class={`inline-flex size-7 items-center justify-center rounded-md border text-[11px] font-semibold transition-colors ${visibilityToggleClass(
										effective.tools,
										runtimeDisabled(assignment, "tools"),
									)}`}
									disabled={runtimeDisabled(assignment, "tools")}
									onclick={() => handleRuntimeToggle(assignment, "tools")}
								>
									<HammerIcon class="size-3.5" />
								</button>
								<button
									type="button"
									title="Console / SSH / IDE"
									aria-label="Toggle console SSH IDE visibility"
									class={`inline-flex size-7 items-center justify-center rounded-md border text-[11px] font-semibold transition-colors ${visibilityToggleClass(
										effective.console,
										runtimeDisabled(assignment, "console"),
									)}`}
									disabled={runtimeDisabled(assignment, "console")}
									onclick={() => handleRuntimeToggle(assignment, "console")}
								>
									<TerminalIcon class="size-3.5" />
								</button>
								<button
									type="button"
									title="Services"
									aria-label="Toggle services visibility"
									class={`inline-flex size-7 items-center justify-center rounded-md border text-[11px] font-semibold transition-colors ${visibilityToggleClass(
										effective.services,
										runtimeDisabled(assignment, "services"),
									)}`}
									disabled={runtimeDisabled(assignment, "services")}
									onclick={() => handleRuntimeToggle(assignment, "services")}
								>
									<ServerIcon class="size-3.5" />
								</button>
								<button
									type="button"
									title="Hooks"
									aria-label="Toggle hooks visibility"
									class={`inline-flex size-7 items-center justify-center rounded-md border text-[11px] font-semibold transition-colors ${visibilityToggleClass(
										effective.hooks,
										runtimeDisabled(assignment, "hooks"),
									)}`}
									disabled={runtimeDisabled(assignment, "hooks")}
									onclick={() => handleRuntimeToggle(assignment, "hooks")}
								>
									<FishingHookIcon class="size-3.5" />
								</button>
								<div class="ml-1 border-l border-border/70 pl-2">
									<Checkbox
										checked={allVisibilityState.checked}
										indeterminate={allVisibilityState.indeterminate}
										disabled={runtimeDisabled(assignment)}
										aria-label="Toggle all session credential visibility"
										onCheckedChange={() =>
											handleAllVisibilityToggle(assignment)}
									/>
								</div>
							</div>
						</div>
						{#if (assignment.uses?.length ?? 0) > 0 && expandedUses[assignmentKey(assignment)]}
							<div class="mt-2 w-full space-y-1.5">
								{#each assignment.uses ?? [] as use (use.id)}
									<div
										class={`flex w-full items-start gap-2 rounded-md border px-2 py-1.5 ${
											useIsExpired(use)
												? "border-border/50 bg-muted/25 text-muted-foreground"
												: "border-border/70 bg-muted/35"
										}`}
									>
										<div class="min-w-0 flex-1">
											<div
												class="whitespace-normal break-words text-[12px] font-medium"
											>
												{use.description}
											</div>
											<div class="text-[11px] text-muted-foreground">
												{formatUseTiming(use)}
											</div>
										</div>
										<button
											type="button"
											class="inline-flex size-6 shrink-0 items-center justify-center rounded-sm text-muted-foreground hover:bg-background hover:text-foreground"
											aria-label="Remove authorized use"
											title="Remove authorized use"
											onclick={() => removeUse(assignment, use.id)}
										>
											<Trash2Icon class="size-3.5" />
										</button>
									</div>
								{/each}
							</div>
						{/if}
					</div>
				{/each}
			</div>
		{/if}
		<DropdownMenuSeparator />
		<div class="p-1">
			<Button
				variant="ghost"
				size="sm"
				class="w-full justify-start gap-2"
				onclick={() => app.ui.openCredentialsDialog()}
			>
				<SettingsIcon class="size-3.5" />
				Manage credentials
			</Button>
		</div>
	</DropdownMenuContent>
</DropdownMenu>

<AlertDialog bind:open={globalVisibilityDialogOpen}>
	<AlertDialogContent>
		<AlertDialogHeader>
			<AlertDialogTitle>Runtime visibility is enabled globally</AlertDialogTitle
			>
			<AlertDialogDescription>
				{#if globalVisibilityDialogAssignment}
					{credentialDisplayName(globalVisibilityDialogAssignment)} is globally enabled
					for
					{globalVisibilityDialogContexts.join(", ")}. To change that, update
					the credential's global visibility settings.
				{/if}
			</AlertDialogDescription>
		</AlertDialogHeader>
		<AlertDialogFooter>
			<AlertDialogCancel onclick={closeGlobalVisibilityDialog}>
				Cancel
			</AlertDialogCancel>
			<AlertDialogAction onclick={openCredentialForGlobalVisibilityEdit}>
				Manage credential
			</AlertDialogAction>
		</AlertDialogFooter>
	</AlertDialogContent>
</AlertDialog>
