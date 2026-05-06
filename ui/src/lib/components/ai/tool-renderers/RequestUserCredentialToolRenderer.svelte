<script lang="ts">
	import KeyRoundIcon from "@lucide/svelte/icons/key-round";
	import {
		ToolContent,
		ToolHeaderControls,
		ToolHeaderStatus,
	} from "$lib/components/ai/tool";
	import {
		type RequestUserCredentialToolInput,
		validateRequestUserCredentialInput,
		validateRequestUserCredentialOutput,
	} from "$lib/components/ai/tool-schemas/requestusercredential-schema";
	import { api } from "$lib/api-client";
	import type {
		CredentialInfo,
		CredentialType,
		GrantedCredential,
		RequestedCredential,
		SessionCredentialAssignment,
	} from "$lib/api-types";
	import { Button } from "$lib/components/ui/button";
	import { CollapsibleTrigger } from "$lib/components/ui/collapsible";
	import { Input } from "$lib/components/ui/input";
	import {
		Select,
		SelectContent,
		SelectItem,
		SelectTrigger,
	} from "$lib/components/ui/select";
	import {
		buildCredentialUseExpiryFromPreset,
		buildAssignmentUses,
		buildGrantedCredentialPayload,
		CUSTOM_CREDENTIAL_OPTION,
		type CredentialValidityPreset,
		type CredentialValidityUnit,
		credentialBindingDescription,
		credentialDisplayName,
		defaultCredentialName,
		findPreferredCredentialId,
		formatApprovedUses,
		listAnyCredentials,
		listOAuthCredentialOptions,
		parseOAuthCredentialOption,
		preferredSourceEnvVar,
	} from "./requestusercredential-helpers";
	import type { ToolRendererComponentProps } from "./types";

	const REQUEST_GRANTED_KEY = "__request_user_credential_granted__";
	const REQUEST_REJECTED_KEY = "__request_user_credential_rejected__";
	const REQUEST_REJECTED_REASON_KEY =
		"__request_user_credential_rejection_reason__";

	type PendingCredentialLike = {
		toolUseID: string;
		credentials: RequestUserCredentialToolInput["credentials"];
	};

	type PendingQuestionResponse = {
		status: "pending" | "answered" | "expired";
		question: PendingCredentialLike | null;
	};

	let {
		toolPart,
		sessionId = null,
		threadId = null,
		onToolApprovalResponse,
		isRaw,
		onToggleRaw,
	}: ToolRendererComponentProps = $props();

	let pendingCredentialRequest = $state<PendingCredentialLike | null>(null);
	let approvalStatus = $state<
		"idle" | "loading" | "pending" | "answered" | "error"
	>("idle");
	let approvalError = $state<string | null>(null);
	let projectCredentials = $state<CredentialInfo[]>([]);
	let credentialTypes = $state<CredentialType[]>([]);
	let sessionAssignments = $state<SessionCredentialAssignment[]>([]);
	let selectedOptionByEnvVar = $state<Record<string, string>>({});
	let createCredentialNamesByEnvVar = $state<Record<string, string>>({});
	let createCredentialSecretsByEnvVar = $state<Record<string, string>>({});
	let validityPresetByEnvVar = $state<Record<string, CredentialValidityPreset>>(
		{},
	);
	let validityValueByEnvVar = $state<Record<string, string>>({});
	let validityUnitByEnvVar = $state<
		Record<string, "hours" | "days" | "weeks" | "never">
	>({});
	let localGrantedCredentials = $state<GrantedCredential[]>([]);
	let rejectionReason = $state("");
	let showRejectionForm = $state(false);
	let isSubmittingApproval = $state(false);
	let isSubmittingRejection = $state(false);

	const validityPresets: Array<{
		value: CredentialValidityPreset;
		label: string;
	}> = [
		{ value: "15_minutes", label: "15 minutes" },
		{ value: "1_hour", label: "1 hour" },
		{ value: "1_day", label: "1 day" },
		{ value: "1_week", label: "1 week" },
		{ value: "custom", label: "Custom" },
	];
	const validityUnits: Array<{
		value: CredentialValidityUnit;
		label: string;
	}> = [
		{ value: "hours", label: "Hours" },
		{ value: "days", label: "Days" },
		{ value: "weeks", label: "Weeks" },
		{ value: "never", label: "Never expires" },
	];

	function getApprovalId(): string | null {
		const approval = toolPart.approval;
		if (approval && typeof approval === "object" && "id" in approval) {
			return typeof approval.id === "string" ? approval.id : null;
		}
		return toolPart.toolCallId || null;
	}

	function initializeDrafts(credentials: RequestedCredential[]) {
		selectedOptionByEnvVar = Object.fromEntries(
			credentials.map((credential) => [
				credential.envVar,
				findPreferredCredentialId(
					credential.envVar,
					projectCredentials,
					sessionAssignments,
				),
			]),
		);
		createCredentialSecretsByEnvVar = {};
		validityPresetByEnvVar = Object.fromEntries(
			credentials.map((credential) => [credential.envVar, "1_hour"]),
		) as Record<string, CredentialValidityPreset>;
		localGrantedCredentials = [];
		rejectionReason = "";
		showRejectionForm = false;
		createCredentialNamesByEnvVar = Object.fromEntries(
			credentials.map((credential) => [
				credential.envVar,
				defaultCredentialName(credential),
			]),
		);
		validityValueByEnvVar = Object.fromEntries(
			credentials.map((credential) => [credential.envVar, "1"]),
		);
		validityUnitByEnvVar = Object.fromEntries(
			credentials.map((credential) => [credential.envVar, "hours"]),
		) as Record<string, "hours" | "days" | "weeks" | "never">;
		isSubmittingApproval = false;
		isSubmittingRejection = false;
	}

	async function fetchPendingQuestion(
		sessionId: string,
		threadId: string,
		questionId: string,
	): Promise<PendingQuestionResponse> {
		return (await api.getThreadChatQuestion(
			sessionId,
			threadId,
			questionId,
		)) as PendingQuestionResponse;
	}

	async function loadCredentialContext() {
		if (!sessionId) {
			projectCredentials = [];
			credentialTypes = [];
			sessionAssignments = [];
			return;
		}
		const [credentialTypesResponse, credentialsResponse, assignmentsResponse] =
			await Promise.all([
				api.getCredentialTypes(),
				api.getCredentials(),
				api.getSessionCredentials(sessionId),
			]);
		credentialTypes = credentialTypesResponse.credentialTypes;
		projectCredentials = credentialsResponse.credentials;
		sessionAssignments = assignmentsResponse.credentials;
	}

	const approvalId = $derived.by(() => getApprovalId());
	const inputValidation = $derived.by(() =>
		validateRequestUserCredentialInput(toolPart.input),
	);
	const validInput = $derived.by(() =>
		inputValidation.success
			? (inputValidation.data as RequestUserCredentialToolInput)
			: undefined,
	);
	const outputValidation = $derived.by(() =>
		toolPart.output !== undefined
			? validateRequestUserCredentialOutput(toolPart.output)
			: null,
	);
	const validOutput = $derived.by(() =>
		outputValidation?.success &&
		outputValidation.data &&
		typeof outputValidation.data === "object" &&
		!Array.isArray(outputValidation.data) &&
		"grantedCredentials" in outputValidation.data
			? (outputValidation.data as { grantedCredentials: GrantedCredential[] })
			: null,
	);
	const outputText = $derived.by(() => {
		if (toolPart.output === undefined || toolPart.output === null) {
			return null;
		}
		if (typeof toolPart.output === "string") {
			return toolPart.output;
		}
		return null;
	});
	const isStreaming = $derived.by(
		() =>
			toolPart.state === "input-streaming" ||
			toolPart.state === "input-available" ||
			toolPart.state === "approval-requested",
	);
	const requestedCredentials = $derived.by(() => validInput?.credentials ?? []);
	const summaryCredentials = $derived.by(
		() => pendingCredentialRequest?.credentials ?? requestedCredentials,
	);
	const grantedCredentials = $derived.by(
		() => validOutput?.grantedCredentials ?? localGrantedCredentials,
	);
	const rejectionSummary = $derived.by(() => {
		const localReason = rejectionReason.trim();
		if (localReason.length > 0) {
			return localReason;
		}
		if (!outputText) {
			return null;
		}
		const rejectionPrefix =
			"The user will not supply the requested credential.";
		if (!outputText.startsWith(rejectionPrefix)) {
			return null;
		}
		const reasonPrefix = `${rejectionPrefix} Reason: `;
		if (outputText.startsWith(reasonPrefix)) {
			return outputText.slice(reasonPrefix.length).trim();
		}
		return "";
	});
	const wasRejected = $derived.by(() => rejectionSummary !== null);
	const grantedCredentialDetails = $derived.by(() =>
		grantedCredentials.map((granted) => {
			const assignment = sessionAssignments.find(
				(item) => item.sessionCredentialId === granted.credentialId,
			);
			const request = requestedCredentials.find(
				(item) => item.envVar === granted.envVar,
			);
			return {
				...granted,
				credentialName:
					assignment?.credential.name ??
					assignment?.credentialId ??
					granted.name ??
					granted.credentialId,
				justification: request?.justification.trim() ?? "",
				uses: granted.approvedUses.map((use) => {
					const assignedUse = assignment?.uses?.find(
						(item) => item.id === use.id,
					);
					return {
						...use,
						expiresAt: assignedUse?.expiresAt,
						createdAt: assignedUse?.createdAt,
					};
				}),
			};
		}),
	);

	function formatCredentialTimeframe(expiresAt: string | undefined): string {
		if (!expiresAt) {
			return "No expiration recorded";
		}
		const expiresTime = new Date(expiresAt).getTime();
		if (!Number.isFinite(expiresTime)) {
			return "Expiration unavailable";
		}
		if (expiresAt.startsWith("9999-12-31")) {
			return "Never expires";
		}
		return `Valid until ${new Intl.DateTimeFormat(undefined, {
			dateStyle: "medium",
			timeStyle: "short",
		}).format(new Date(expiresAt))}`;
	}

	$effect(() => {
		if (toolPart.state !== "approval-requested") {
			approvalStatus = "idle";
			approvalError = null;
			pendingCredentialRequest = null;
			credentialTypes = [];
			rejectionReason = "";
			showRejectionForm = false;
			isSubmittingApproval = false;
			isSubmittingRejection = false;
			return;
		}

		approvalError = null;
		showRejectionForm = false;
		isSubmittingApproval = false;
		isSubmittingRejection = false;

		if (requestedCredentials.length > 0) {
			const nextRequest = {
				toolUseID: approvalId ?? toolPart.toolCallId,
				credentials: requestedCredentials,
			};
			pendingCredentialRequest = nextRequest;
			approvalStatus = "loading";
			void loadCredentialContext()
				.then(() => {
					initializeDrafts(nextRequest.credentials);
					approvalStatus = "pending";
				})
				.catch((error) => {
					approvalStatus = "error";
					approvalError =
						error instanceof Error
							? error.message
							: "Failed to load credentials";
				});
			return;
		}

		if (!sessionId || !threadId || !approvalId) {
			approvalStatus = "loading";
			pendingCredentialRequest = null;
			return;
		}

		approvalStatus = "loading";
		pendingCredentialRequest = null;
		let cancelled = false;
		void fetchPendingQuestion(sessionId, threadId, approvalId)
			.then(async (result) => {
				if (cancelled) {
					return;
				}
				if (
					result.status === "pending" &&
					result.question &&
					Array.isArray(result.question.credentials) &&
					result.question.credentials.length > 0
				) {
					pendingCredentialRequest = result.question;
					try {
						await loadCredentialContext();
						initializeDrafts(result.question.credentials);
						if (!cancelled) {
							approvalStatus = "pending";
						}
					} catch (error) {
						if (!cancelled) {
							approvalStatus = "error";
							approvalError =
								error instanceof Error
									? error.message
									: "Failed to load credentials";
						}
					}
					return;
				}
				pendingCredentialRequest = null;
				approvalStatus = "answered";
			})
			.catch((error) => {
				if (cancelled) {
					return;
				}
				approvalStatus = "error";
				approvalError =
					error instanceof Error
						? error.message
						: "Failed to load credential request";
			});

		return () => {
			cancelled = true;
		};
	});
	$effect(() => {
		if (!sessionId || grantedCredentials.length === 0) {
			return;
		}

		let cancelled = false;
		api
			.getSessionCredentials(sessionId)
			.then((response) => {
				if (!cancelled) {
					sessionAssignments = response.credentials;
				}
			})
			.catch(() => {
				if (!cancelled) {
					sessionAssignments = [];
				}
			});

		return () => {
			cancelled = true;
		};
	});

	async function assignCredentialToSession(
		request: RequestedCredential,
		credential: CredentialInfo,
		sourceEnvVar: string,
		expiresAt?: string,
	) {
		if (!sessionId) {
			throw new Error("Missing session context");
		}
		const existing = sessionAssignments.find(
			(assignment) => assignment.credentialId === credential.id,
		);
		const persistedAssignments = sessionAssignments.filter(
			(assignment) =>
				assignment.credentialId === credential.id ||
				Boolean(assignment.sessionCredentialId) ||
				Boolean(assignment.envVar) ||
				Boolean(assignment.sourceEnvVar) ||
				(assignment.uses?.length ?? 0) > 0 ||
				assignment.agentVisible !== assignment.credential.agentVisible,
		);
		const response = await api.setSessionCredentials(sessionId, [
			...persistedAssignments
				.filter((assignment) => assignment.credentialId !== credential.id)
				.map((assignment) => ({
					credentialId: assignment.credentialId,
					sessionCredentialId: assignment.sessionCredentialId,
					envVar: assignment.envVar,
					sourceEnvVar: assignment.sourceEnvVar,
					agentVisible: assignment.agentVisible,
					uses: assignment.uses,
				})),
			{
				credentialId: credential.id,
				sessionCredentialId: existing?.sessionCredentialId,
				envVar: request.envVar,
				sourceEnvVar,
				agentVisible: true,
				uses: [
					...(existing?.uses ?? []),
					...buildAssignmentUses(request, expiresAt),
				],
			},
		]);
		sessionAssignments = response.credentials;
		if (typeof window !== "undefined") {
			window.dispatchEvent(
				new CustomEvent("discobot:session-credentials-changed", {
					detail: { sessionId },
				}),
			);
		}
	}

	async function submitGrantedCredentials(
		selectedCredentialIds: Record<string, string>,
	) {
		approvalError = null;
		if (!threadId || !pendingCredentialRequest || !sessionId) {
			approvalStatus = "pending";
			approvalError = "Missing thread context";
			return;
		}
		const payload = buildGrantedCredentialPayload(
			pendingCredentialRequest.credentials,
			selectedCredentialIds,
			sessionAssignments,
		);
		localGrantedCredentials = payload.grantedCredentials;
		try {
			await api.submitThreadChatAnswer(sessionId, threadId, {
				toolUseID: pendingCredentialRequest.toolUseID,
				answers: {
					[REQUEST_GRANTED_KEY]: JSON.stringify(payload),
				},
			});
			onToolApprovalResponse?.({
				id: pendingCredentialRequest.toolUseID,
				approved: true,
			});
			approvalStatus = "answered";
			pendingCredentialRequest = null;
		} catch (error) {
			approvalStatus = "pending";
			approvalError =
				error instanceof Error
					? error.message
					: "Failed to submit credential response";
		}
	}

	async function submitCredentialRejection(reason: string) {
		approvalError = null;
		const trimmedReason =
			reason.trim() || "User denied the credential request.";
		if (!threadId || !pendingCredentialRequest || !sessionId) {
			approvalStatus = "pending";
			approvalError = "Missing thread context";
			return;
		}
		isSubmittingRejection = true;
		try {
			await api.submitThreadChatAnswer(sessionId, threadId, {
				toolUseID: pendingCredentialRequest.toolUseID,
				answers: {
					[REQUEST_REJECTED_KEY]: "true",
					[REQUEST_REJECTED_REASON_KEY]: trimmedReason,
				},
			});
			onToolApprovalResponse?.({
				id: pendingCredentialRequest.toolUseID,
				approved: false,
				reason: trimmedReason,
			});
			rejectionReason = trimmedReason;
			showRejectionForm = false;
			approvalStatus = "answered";
			pendingCredentialRequest = null;
		} catch (error) {
			approvalStatus = "pending";
			approvalError =
				error instanceof Error
					? error.message
					: "Failed to reject credential request";
		} finally {
			isSubmittingRejection = false;
		}
	}

	async function approveCredentialRequest() {
		approvalError = null;
		if (!pendingCredentialRequest) {
			return;
		}
		isSubmittingApproval = true;
		try {
			const resolvedCredentialIds: Record<string, string> = {};
			for (const request of pendingCredentialRequest.credentials) {
				const selectedOption =
					selectedOptionByEnvVar[request.envVar]?.trim() ?? "";
				if (!selectedOption) {
					throw new Error(`Select a credential for ${request.envVar}.`);
				}
				const selectedOAuthType = parseOAuthCredentialOption(
					selectedOption,
					credentialTypes,
				);
				if (selectedOAuthType) {
					throw new Error(
						`${selectedOAuthType.name} OAuth isn't wired up here yet.`,
					);
				}
				const expiresAt = buildCredentialUseExpiryFromPreset(
					validityPresetByEnvVar[request.envVar] ?? "1_hour",
					validityValueByEnvVar[request.envVar] ?? "1",
					validityUnitByEnvVar[request.envVar] ?? "hours",
				);
				if (selectedOption === CUSTOM_CREDENTIAL_OPTION) {
					const value =
						createCredentialSecretsByEnvVar[request.envVar]?.trim() ?? "";
					if (!value) {
						throw new Error(`Enter a credential value for ${request.envVar}.`);
					}
					const credential = await api.createCredential({
						name:
							createCredentialNamesByEnvVar[request.envVar]?.trim() ||
							defaultCredentialName(request),
						description: request.justification.trim() || undefined,
						authType: "api_key",
						envVars: [{ key: request.envVar, value }],
						agentVisible: false,
					});
					projectCredentials = [...projectCredentials, credential];
					await assignCredentialToSession(
						request,
						credential,
						request.envVar,
						expiresAt,
					);
					createCredentialSecretsByEnvVar = {
						...createCredentialSecretsByEnvVar,
						[request.envVar]: "",
					};
					resolvedCredentialIds[request.envVar] = credential.id;
					continue;
				}
				const credential = projectCredentials.find(
					(item) => item.id === selectedOption,
				);
				if (!credential) {
					throw new Error(`Credential for ${request.envVar} was not found.`);
				}
				const sourceEnvVar = preferredSourceEnvVar(request.envVar, credential);
				if (!sourceEnvVar) {
					throw new Error(
						`Credential ${credentialDisplayName(credential)} has no usable environment variable binding.`,
					);
				}
				await assignCredentialToSession(
					request,
					credential,
					sourceEnvVar,
					expiresAt,
				);
				resolvedCredentialIds[request.envVar] = credential.id;
			}
			await submitGrantedCredentials(resolvedCredentialIds);
		} catch (error) {
			approvalError =
				error instanceof Error
					? error.message
					: "Failed to approve credential request";
		} finally {
			isSubmittingApproval = false;
		}
	}
</script>

<div class="flex items-center justify-between gap-4 px-4 pt-4">
	<CollapsibleTrigger
		class="flex min-w-0 flex-1 items-center gap-2 text-left text-amber-700 dark:text-amber-300"
	>
		<KeyRoundIcon class="size-4 shrink-0 text-amber-600 dark:text-amber-300" />
		<span class="truncate font-medium text-sm">Credential request</span>
		<ToolHeaderStatus state={toolPart.state} />
	</CollapsibleTrigger>
	<ToolHeaderControls {isRaw} {onToggleRaw} />
</div>

<ToolContent>
	{#if wasRejected}
		<div class="space-y-4 p-4 pt-3">
			<div class="space-y-1">
				<p class="font-medium text-sm">Credential request rejected</p>
				{#if rejectionSummary}
					<p class="text-muted-foreground text-sm">{rejectionSummary}</p>
				{/if}
			</div>
		</div>
	{:else if grantedCredentialDetails.length > 0}
		<div class="space-y-4 p-4 pt-3">
			<div class="space-y-1">
				<p class="font-medium text-sm">Credential access granted</p>
				<p class="text-muted-foreground text-sm">
					The agent can use these credentials for the approved purposes below.
				</p>
			</div>
			{#each grantedCredentialDetails as granted}
				<div class="space-y-3 rounded-md border border-amber-500/30 p-3">
					<div class="space-y-1">
						<p class="font-medium text-sm">{granted.credentialName}</p>
						<p class="font-mono text-xs text-muted-foreground">
							{granted.envVar} → {granted.credentialId}
						</p>
					</div>
					{#if granted.justification}
						<div class="space-y-1 rounded-md bg-muted/30 p-2">
							<p
								class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
							>
								Purpose
							</p>
							<p class="text-sm">{granted.justification}</p>
						</div>
					{/if}
					{#if granted.uses.length > 0}
						<div class="space-y-2">
							<p
								class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
							>
								Approved uses
							</p>
							<ul class="space-y-2">
								{#each granted.uses as use}
									<li class="space-y-1 rounded-md bg-muted/30 p-2">
										<p class="text-sm">{use.description}</p>
										<p class="text-muted-foreground text-xs">
											{formatCredentialTimeframe(use.expiresAt)}
										</p>
										<p class="font-mono text-muted-foreground text-[11px]">
											Use ID: {use.id}
										</p>
									</li>
								{/each}
							</ul>
						</div>
					{:else}
						<p class="text-muted-foreground text-sm">
							No approved-use details were recorded.
						</p>
					{/if}
				</div>
			{/each}
		</div>
	{:else if toolPart.state === "approval-requested"}
		<div class="space-y-4 p-4 pt-3">
			{#if approvalStatus === "loading"}
				<p class="text-muted-foreground text-sm">
					Loading credential request...
				</p>
			{:else if approvalStatus === "error"}
				<p class="text-destructive text-sm">
					{approvalError ?? "Failed to load credential request"}
				</p>
			{:else if approvalStatus === "answered"}
				<p class="text-muted-foreground text-sm">
					Credential request answered.
				</p>
			{:else if pendingCredentialRequest}
				<div class="space-y-4 rounded-lg border bg-card p-4">
					<div>
						<h3 class="font-semibold text-base">Approve credential access</h3>
						<p class="text-muted-foreground text-sm">
							Review why the agent is asking, choose which credential to use,
							and approve or deny the request.
						</p>
					</div>

					{#each pendingCredentialRequest.credentials as request (request.envVar)}
						{@const preferredId = findPreferredCredentialId(
							request.envVar,
							projectCredentials,
							sessionAssignments,
						)}
						{@const selectedOption =
							selectedOptionByEnvVar[request.envVar] ?? preferredId}
						{@const selectedCredential = projectCredentials.find(
							(item) => item.id === selectedOption,
						)}
						{@const selectedOAuthType = parseOAuthCredentialOption(
							selectedOption,
							credentialTypes,
						)}
						{@const oauthOptions = listOAuthCredentialOptions(
							request.envVar,
							credentialTypes,
							projectCredentials,
						)}
						<div
							class="space-y-4 rounded-md border border-border bg-background p-4"
						>
							<div class="space-y-3">
								<div class="space-y-1">
									<p class="font-semibold text-sm">{request.name}</p>
									<p class="font-mono text-xs text-muted-foreground">
										{request.envVar}
									</p>
								</div>

								<div class="space-y-2 rounded-md bg-muted/40 p-3">
									<p
										class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
									>
										Why access is needed
									</p>
									<p class="text-sm">{request.justification}</p>
								</div>

								{#if formatApprovedUses(request).length > 0}
									<div class="space-y-2 rounded-md bg-muted/40 p-3">
										<p
											class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
										>
											Allowed uses
										</p>
										<ul class="list-disc space-y-1 pl-5 text-sm">
											{#each formatApprovedUses(request) as futureUse}
												<li>{futureUse}</li>
											{/each}
										</ul>
									</div>
								{/if}
							</div>

							<div class="space-y-2">
								<p
									class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
								>
									Credential to use
								</p>
								<Select
									type="single"
									value={selectedOption}
									onValueChange={(value) => {
										selectedOptionByEnvVar = {
											...selectedOptionByEnvVar,
											[request.envVar]: value,
										};
									}}
								>
									<SelectTrigger class="w-full">
										{selectedOption === CUSTOM_CREDENTIAL_OPTION
											? "Custom credential"
											: selectedOAuthType
												? `New ${selectedOAuthType.name} OAuth`
												: selectedCredential
													? credentialDisplayName(selectedCredential)
													: "Choose a credential"}
									</SelectTrigger>
									<SelectContent>
										{#each listAnyCredentials(projectCredentials, sessionAssignments) as match (match.credential.id)}
											<SelectItem value={match.credential.id}>
												{credentialDisplayName(match.credential)}
											</SelectItem>
										{/each}
										{#each oauthOptions as option (option.value)}
											<SelectItem value={option.value}>
												{option.label}
											</SelectItem>
										{/each}
										<SelectItem value={CUSTOM_CREDENTIAL_OPTION}
											>Custom credential</SelectItem
										>
									</SelectContent>
								</Select>
								{#if selectedCredential}
									<p class="text-muted-foreground text-xs">
										{credentialBindingDescription(
											request.envVar,
											selectedCredential,
										)}
									</p>
								{:else if selectedOAuthType}
									<div
										class="space-y-2 rounded-md border border-dashed border-border p-3"
									>
										<p
											class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
										>
											OAuth sign-in
										</p>
										<p class="text-sm">
											We'll start the {selectedOAuthType.name} OAuth flow here next.
										</p>
									</div>
								{/if}
							</div>

							<div class="space-y-2">
								<p
									class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
								>
									How long it is valid
								</p>
								<Select
									type="single"
									value={validityPresetByEnvVar[request.envVar] ?? "1_hour"}
									onValueChange={(value) => {
										validityPresetByEnvVar = {
											...validityPresetByEnvVar,
											[request.envVar]: value as CredentialValidityPreset,
										};
										if (value === "custom") {
											validityValueByEnvVar = {
												...validityValueByEnvVar,
												[request.envVar]:
													validityValueByEnvVar[request.envVar] ?? "1",
											};
											validityUnitByEnvVar = {
												...validityUnitByEnvVar,
												[request.envVar]:
													validityUnitByEnvVar[request.envVar] ?? "hours",
											};
										}
									}}
								>
									<SelectTrigger class="w-full">
										{validityPresets.find(
											(preset) =>
												preset.value ===
												(validityPresetByEnvVar[request.envVar] ?? "1_hour"),
										)?.label ?? "1 hour"}
									</SelectTrigger>
									<SelectContent>
										{#each validityPresets as preset (preset.value)}
											<SelectItem value={preset.value}
												>{preset.label}</SelectItem
											>
										{/each}
									</SelectContent>
								</Select>
							</div>

							{#if (validityPresetByEnvVar[request.envVar] ?? "1_hour") === "custom"}
								<div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_10rem]">
									<div class="space-y-2">
										<label
											class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
											for={`validity-${request.envVar}`}
										>
											Custom duration
										</label>
										<Input
											id={`validity-${request.envVar}`}
											type="number"
											min="1"
											disabled={(validityUnitByEnvVar[request.envVar] ??
												"hours") === "never"}
											value={validityValueByEnvVar[request.envVar] ?? "1"}
											oninput={(event) => {
												validityValueByEnvVar = {
													...validityValueByEnvVar,
													[request.envVar]: (
														event.currentTarget as HTMLInputElement
													).value,
												};
											}}
										/>
									</div>
									<div class="space-y-2">
										<p
											class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
										>
											Unit
										</p>
										<Select
											type="single"
											value={validityUnitByEnvVar[request.envVar] ?? "hours"}
											onValueChange={(value) => {
												validityUnitByEnvVar = {
													...validityUnitByEnvVar,
													[request.envVar]: value as CredentialValidityUnit,
												};
											}}
										>
											<SelectTrigger class="w-full">
												{validityUnits.find(
													(unit) =>
														unit.value ===
														(validityUnitByEnvVar[request.envVar] ?? "hours"),
												)?.label ?? "Hours"}
											</SelectTrigger>
											<SelectContent>
												{#each validityUnits as unit (unit.value)}
													<SelectItem value={unit.value}
														>{unit.label}</SelectItem
													>
												{/each}
											</SelectContent>
										</Select>
									</div>
								</div>
							{/if}

							{#if selectedOption === CUSTOM_CREDENTIAL_OPTION}
								<div
									class="space-y-3 rounded-md border border-dashed border-border p-3"
								>
									<p
										class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
									>
										Enter new credential
									</p>
									<Input
										value={createCredentialNamesByEnvVar[request.envVar] ??
											defaultCredentialName(request)}
										placeholder="Credential name"
										oninput={(event) => {
											createCredentialNamesByEnvVar = {
												...createCredentialNamesByEnvVar,
												[request.envVar]: (
													event.currentTarget as HTMLInputElement
												).value,
											};
										}}
									/>
									<Input
										type="password"
										value={createCredentialSecretsByEnvVar[request.envVar] ??
											""}
										placeholder={`Enter ${request.envVar}`}
										class="font-mono"
										oninput={(event) => {
											createCredentialSecretsByEnvVar = {
												...createCredentialSecretsByEnvVar,
												[request.envVar]: (
													event.currentTarget as HTMLInputElement
												).value,
											};
										}}
									/>
								</div>
							{/if}
						</div>
					{/each}

					<div class="flex flex-wrap justify-end gap-2">
						{#if showRejectionForm}
							<div
								class="w-full space-y-2 rounded-md border border-dashed border-border p-3"
							>
								<p
									class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
								>
									Why are you denying this request?
								</p>
								<textarea
									class="min-h-20 w-full resize-y rounded-md border border-input bg-background px-3 py-2 text-sm outline-none placeholder:text-muted-foreground focus:border-primary focus:ring-1 focus:ring-primary/30"
									disabled={isSubmittingRejection || isSubmittingApproval}
									oninput={(event) => {
										rejectionReason = (
											event.currentTarget as HTMLTextAreaElement
										).value;
									}}
									placeholder="Optional: explain why you are denying this request"
									value={rejectionReason}
								></textarea>
								<div class="flex flex-wrap justify-end gap-2">
									<Button
										variant="ghost"
										disabled={isSubmittingApproval || isSubmittingRejection}
										onclick={() => {
											showRejectionForm = false;
										}}
									>
										Cancel
									</Button>
									<Button
										variant="outline"
										disabled={isSubmittingApproval || isSubmittingRejection}
										onclick={() => {
											void submitCredentialRejection(rejectionReason);
										}}
									>
										{isSubmittingRejection ? "Denying..." : "Confirm deny"}
									</Button>
								</div>
							</div>
						{:else}
							<Button
								variant="outline"
								disabled={isSubmittingApproval || isSubmittingRejection}
								onclick={() => {
									approvalError = null;
									showRejectionForm = true;
								}}
							>
								Deny
							</Button>
							<Button
								disabled={isSubmittingApproval || isSubmittingRejection}
								onclick={() => {
									void approveCredentialRequest();
								}}
							>
								{isSubmittingApproval ? "Approving..." : "Approve"}
							</Button>
						{/if}
					</div>
				</div>
			{:else}
				<p class="text-muted-foreground text-sm">
					Waiting for credential request details...
				</p>
			{/if}

			{#if approvalError && approvalStatus !== "error"}
				<p class="text-destructive text-sm">{approvalError}</p>
			{/if}
		</div>
	{:else if !toolPart.input || typeof toolPart.input !== "object"}
		<div class="p-4 pt-3 text-muted-foreground text-sm">
			{isStreaming ? "Loading credential request..." : "No input data"}
		</div>
	{:else if !inputValidation.success}
		<div class="space-y-3 p-4 pt-3">
			<p class="text-muted-foreground text-sm">
				{isStreaming
					? "Loading credential request..."
					: "Could not parse credential request details."}
			</p>
			{#if toolPart.errorText}
				<p class="text-destructive text-sm">{toolPart.errorText}</p>
			{/if}
		</div>
	{:else}
		<div class="space-y-4 p-4 pt-3">
			{#if requestedCredentials.length > 0}
				<div class="space-y-3">
					{#each requestedCredentials as request}
						<div class="space-y-1">
							<p class="font-medium text-sm">{request.name}</p>
							<p class="text-muted-foreground text-sm">
								{request.justification}
							</p>
							<p class="text-muted-foreground text-xs">
								<span class="font-mono">{request.envVar}</span>
							</p>
							{#if formatApprovedUses(request).length > 0}
								<ul
									class="list-disc space-y-1 pl-5 text-muted-foreground text-sm"
								>
									{#each formatApprovedUses(request) as futureUse}
										<li>{futureUse}</li>
									{/each}
								</ul>
							{/if}
						</div>
					{/each}
				</div>
			{/if}

			{#if wasRejected}
				<div class="space-y-1.5">
					<h4
						class="font-medium text-muted-foreground text-xs uppercase tracking-wide"
					>
						Status
					</h4>
					<div class="space-y-1">
						<p class="font-medium text-sm">Credential request rejected</p>
						{#if rejectionSummary}
							<p class="text-muted-foreground text-sm">{rejectionSummary}</p>
						{/if}
					</div>
				</div>
			{:else if grantedCredentials.length > 0}
				<div class="space-y-1.5">
					<h4
						class="font-medium text-muted-foreground text-xs uppercase tracking-wide"
					>
						Granted ids
					</h4>
					{#each grantedCredentials as granted}
						<div class="space-y-1 rounded-md border bg-muted/30 p-3 text-sm">
							<p class="font-medium">{granted.name}</p>
							<p class="font-mono text-xs text-muted-foreground">
								{granted.envVar} → {granted.credentialId}
							</p>
						</div>
					{/each}
				</div>
			{/if}

			{#if outputValidation && !outputValidation.success}
				<div
					class="rounded-md border border-dashed px-3 py-2 text-muted-foreground text-xs"
				>
					Could not parse tool output.
				</div>
			{/if}

			{#if toolPart.errorText}
				<p class="text-destructive text-sm">{toolPart.errorText}</p>
			{/if}
		</div>
	{/if}
</ToolContent>
