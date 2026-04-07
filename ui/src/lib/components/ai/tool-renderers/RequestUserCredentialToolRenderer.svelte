<script lang="ts">
	import CheckIcon from "@lucide/svelte/icons/check";
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
		buildAssignmentUses,
		buildGrantedCredentialPayload,
		credentialBindingDescription,
		credentialDisplayName,
		defaultCredentialName,
		findCredentialMatches,
		formatApprovedUses,
		listAnyCredentials,
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
	let sessionAssignments = $state<SessionCredentialAssignment[]>([]);
	let selectedCredentialIdsByEnvVar = $state<Record<string, string>>({});
	let selectedCredentialLabelsByEnvVar = $state<Record<string, string>>({});
	let selectedAnyCredentialIdsByEnvVar = $state<Record<string, string>>({});
	let selectedModeByEnvVar = $state<
		Record<string, "existing" | "create" | "reject">
	>({});
	let createCredentialNamesByEnvVar = $state<Record<string, string>>({});
	let createCredentialSecretsByEnvVar = $state<Record<string, string>>({});
	let localGrantedCredentials = $state<GrantedCredential[]>([]);
	let rejectionReason = $state("");
	let actionEnvVar = $state<string | null>(null);

	function getApprovalId(): string | null {
		const approval = toolPart.approval;
		if (approval && typeof approval === "object" && "id" in approval) {
			return typeof approval.id === "string" ? approval.id : null;
		}
		return toolPart.toolCallId || null;
	}

	function initializeDrafts(credentials: RequestedCredential[]) {
		selectedCredentialIdsByEnvVar = {};
		selectedCredentialLabelsByEnvVar = {};
		selectedAnyCredentialIdsByEnvVar = {};
		selectedModeByEnvVar = Object.fromEntries(
			credentials.map((credential) => [credential.envVar, "existing"]),
		) as Record<string, "existing" | "create" | "reject">;
		createCredentialSecretsByEnvVar = {};
		localGrantedCredentials = [];
		rejectionReason = "";
		createCredentialNamesByEnvVar = Object.fromEntries(
			credentials.map((credential) => [
				credential.envVar,
				defaultCredentialName(credential),
			]),
		);
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
			sessionAssignments = [];
			return;
		}
		const [credentialsResponse, assignmentsResponse] = await Promise.all([
			api.getCredentials(),
			api.getSessionCredentials(sessionId),
		]);
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
	const allCredentialsResolved = $derived.by(() =>
		summaryCredentials.length > 0
			? summaryCredentials.every(
					(credential) =>
						(selectedCredentialIdsByEnvVar[credential.envVar]?.trim().length ??
							0) > 0,
				)
			: false,
	);

	$effect(() => {
		if (toolPart.state !== "approval-requested") {
			approvalStatus = "idle";
			approvalError = null;
			pendingCredentialRequest = null;
			sessionAssignments = [];
			localGrantedCredentials = [];
			rejectionReason = "";
			actionEnvVar = null;
			return;
		}

		approvalError = null;
		actionEnvVar = null;

		if (requestedCredentials.length > 0) {
			const nextRequest = {
				toolUseID: approvalId ?? toolPart.toolCallId,
				credentials: requestedCredentials,
			};
			pendingCredentialRequest = nextRequest;
			approvalStatus = "loading";
			initializeDrafts(nextRequest.credentials);
			void loadCredentialContext()
				.then(() => {
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
					initializeDrafts(result.question.credentials);
					try {
						await loadCredentialContext();
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

	async function assignCredentialToSession(
		request: RequestedCredential,
		credential: CredentialInfo,
		sourceEnvVar: string,
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
				uses: [...(existing?.uses ?? []), ...buildAssignmentUses(request)],
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

	async function submitGrantedCredentials() {
		approvalError = null;
		if (!threadId || !pendingCredentialRequest || !sessionId) {
			approvalStatus = "pending";
			approvalError = "Missing thread context";
			return;
		}
		const payload = buildGrantedCredentialPayload(
			pendingCredentialRequest.credentials,
			selectedCredentialIdsByEnvVar,
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
		const trimmedReason = reason.trim();
		if (!trimmedReason) {
			approvalError = "Add a reason before rejecting the credential request.";
			return;
		}
		if (!threadId || !pendingCredentialRequest || !sessionId) {
			approvalStatus = "pending";
			approvalError = "Missing thread context";
			return;
		}
		actionEnvVar = "__reject__";
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
			approvalStatus = "answered";
			pendingCredentialRequest = null;
		} catch (error) {
			approvalStatus = "pending";
			approvalError =
				error instanceof Error
					? error.message
					: "Failed to reject credential request";
		} finally {
			actionEnvVar = null;
		}
	}

	async function finalizeCredentialSelection(
		request: RequestedCredential,
		credentialId: string,
		label: string,
	) {
		selectedCredentialIdsByEnvVar = {
			...selectedCredentialIdsByEnvVar,
			[request.envVar]: credentialId,
		};
		selectedCredentialLabelsByEnvVar = {
			...selectedCredentialLabelsByEnvVar,
			[request.envVar]: label,
		};
		if (
			summaryCredentials.length > 0 &&
			summaryCredentials.every(
				(credential) =>
					(selectedCredentialIdsByEnvVar[credential.envVar]?.trim().length ??
						0) > 0 || credential.envVar === request.envVar,
			)
		) {
			await submitGrantedCredentials();
		}
	}

	async function useExistingCredential(
		request: RequestedCredential,
		credentialId: string,
		sourceEnvVar: string,
	) {
		actionEnvVar = request.envVar;
		approvalError = null;
		try {
			const credential = projectCredentials.find(
				(item) => item.id === credentialId,
			);
			if (!credential) {
				throw new Error("Credential not found");
			}
			await assignCredentialToSession(request, credential, sourceEnvVar);
			await finalizeCredentialSelection(
				request,
				credential.id,
				credentialDisplayName(credential),
			);
		} catch (error) {
			approvalError =
				error instanceof Error ? error.message : "Failed to use credential";
		} finally {
			actionEnvVar = null;
		}
	}

	async function useSelectedAnyCredential(request: RequestedCredential) {
		const credentialId =
			selectedAnyCredentialIdsByEnvVar[request.envVar]?.trim();
		if (!credentialId) {
			approvalError = "Choose an existing credential first.";
			return;
		}
		const credential = projectCredentials.find(
			(item) => item.id === credentialId,
		);
		if (!credential) {
			approvalError = "Credential not found";
			return;
		}
		const sourceEnvVar = preferredSourceEnvVar(request.envVar, credential);
		if (!sourceEnvVar) {
			approvalError =
				"This credential has no usable environment variable binding.";
			return;
		}
		await useExistingCredential(request, credentialId, sourceEnvVar);
	}

	async function createAndUseCredential(request: RequestedCredential) {
		actionEnvVar = request.envVar;
		approvalError = null;
		try {
			const name =
				createCredentialNamesByEnvVar[request.envVar]?.trim() ||
				defaultCredentialName(request);
			const value =
				createCredentialSecretsByEnvVar[request.envVar]?.trim() || "";
			if (!value) {
				throw new Error(
					"Enter a credential value before creating a new credential.",
				);
			}
			const credential = await api.createCredential({
				name,
				description: request.justification.trim() || undefined,
				authType: "api_key",
				envVars: [{ key: request.envVar, value }],
				agentVisible: false,
			});
			projectCredentials = [...projectCredentials, credential];
			await assignCredentialToSession(request, credential, request.envVar);
			createCredentialSecretsByEnvVar = {
				...createCredentialSecretsByEnvVar,
				[request.envVar]: "",
			};
			await finalizeCredentialSelection(
				request,
				credential.id,
				credentialDisplayName(credential),
			);
		} catch (error) {
			approvalError =
				error instanceof Error ? error.message : "Failed to create credential";
		} finally {
			actionEnvVar = null;
		}
	}
</script>

<div class="flex items-center justify-between gap-4 px-4 pt-4">
	<CollapsibleTrigger
		class="flex min-w-0 flex-1 items-center gap-2 text-left text-muted-foreground"
		disabled={true}
	>
		<KeyRoundIcon class="size-4 shrink-0 text-muted-foreground" />
		<span class="truncate font-medium text-sm">Credential request</span>
		<ToolHeaderStatus state={toolPart.state} />
	</CollapsibleTrigger>
	<ToolHeaderControls {isRaw} {onToggleRaw} canCollapse={false} />
</div>

<ToolContent>
	{#if toolPart.state === "approval-requested"}
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
				{#if wasRejected}
					<div class="space-y-1">
						<p class="font-medium text-sm">Credential request rejected</p>
						{#if rejectionSummary}
							<p class="text-muted-foreground text-sm">{rejectionSummary}</p>
						{/if}
					</div>
				{:else if grantedCredentials.length > 0}
					<div class="space-y-3">
						{#each grantedCredentials as granted}
							<div class="space-y-1">
								<p class="font-medium text-sm">{granted.name}</p>
								<p class="font-mono text-xs text-muted-foreground">
									{granted.envVar} → {granted.credentialId}
								</p>
								{#if granted.approvedUses && granted.approvedUses.length > 0}
									<ul
										class="list-disc space-y-1 pl-5 text-muted-foreground text-sm"
									>
										{#each granted.approvedUses as use}
											<li>
												{use.description}
												<span class="font-mono text-xs">({use.id})</span>
											</li>
										{/each}
									</ul>
								{/if}
							</div>
						{/each}
					</div>
				{:else}
					<p class="text-muted-foreground text-sm">
						Credential request answered.
					</p>
				{/if}
			{:else if pendingCredentialRequest}
				<div class="space-y-3 rounded-lg border bg-card p-4">
					<div>
						<h3 class="font-semibold text-base">Agent needs a credential</h3>
						<p class="text-muted-foreground text-sm">
							Choose an existing credential that matches the requested
							environment variable or create a new one.
						</p>
					</div>

					{#each pendingCredentialRequest.credentials as request (request.envVar)}
						{@const resolvedLabel =
							selectedCredentialLabelsByEnvVar[request.envVar]}
						{@const matches = findCredentialMatches(
							request.envVar,
							projectCredentials,
							sessionAssignments,
						)}
						{@const exactMatchIds = new Set(
							matches.map((match) => match.credential.id),
						)}
						{@const anyCredentials = listAnyCredentials(
							projectCredentials,
							sessionAssignments,
						).filter((match) => !exactMatchIds.has(match.credential.id))}
						<div
							class="space-y-3 rounded-md border border-border bg-background p-3"
						>
							<div class="space-y-1">
								<div class="flex items-center justify-between gap-3">
									<p class="font-medium text-sm">{request.name}</p>
									{#if resolvedLabel}
										<span
											class="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2 py-0.5 text-primary text-xs"
										>
											<CheckIcon class="size-3" />
											Assigned
										</span>
									{/if}
								</div>
								<p class="text-muted-foreground text-sm">
									{request.justification}
								</p>
								<p class="text-muted-foreground text-xs">
									Env var: <span class="font-mono">{request.envVar}</span>
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

							{#if resolvedLabel}
								<p class="text-muted-foreground text-sm">
									Using {resolvedLabel} in this session.
								</p>
							{:else}
								{@const selectedMode =
									selectedModeByEnvVar[request.envVar] ?? "existing"}
								{@const selectedAnyCredential = projectCredentials.find(
									(item) =>
										item.id ===
										selectedAnyCredentialIdsByEnvVar[request.envVar],
								)}
								<div class="space-y-3">
									<div class="space-y-2">
										<p
											class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
										>
											Choose how to continue
										</p>
										<Select
											type="single"
											value={selectedMode}
											onValueChange={(value) => {
												selectedModeByEnvVar = {
													...selectedModeByEnvVar,
													[request.envVar]: value as
														| "existing"
														| "create"
														| "reject",
												};
											}}
										>
											<SelectTrigger class="w-full">
												{selectedMode === "create"
													? "Enter new one"
													: selectedMode === "reject"
														? "Reject"
														: "Select existing"}
											</SelectTrigger>
											<SelectContent>
												<SelectItem value="existing">Select existing</SelectItem
												>
												<SelectItem value="create">Enter new one</SelectItem>
												<SelectItem value="reject">Reject</SelectItem>
											</SelectContent>
										</Select>
									</div>

									{#if selectedMode === "existing"}
										<div class="space-y-2">
											<p
												class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
											>
												Use existing matching credential
											</p>
											{#if matches.length === 0}
												<p class="text-muted-foreground text-sm">
													No existing credential exposes <span class="font-mono"
														>{request.envVar}</span
													>.
												</p>
											{:else}
												<div class="space-y-2">
													{#each matches as match (match.credential.id)}
														<div
															class="flex items-center justify-between gap-3 rounded-md border border-border p-2"
														>
															<div class="min-w-0 flex-1">
																<p class="truncate font-medium text-sm">
																	{credentialDisplayName(match.credential)}
																</p>
																<p
																	class="truncate text-muted-foreground text-xs"
																>
																	{match.assigned
																		? "Already assigned to this session"
																		: "Exact env var match"}
																</p>
															</div>
															<Button
																variant="outline"
																size="sm"
																disabled={actionEnvVar === request.envVar}
																onclick={() => {
																	void useExistingCredential(
																		request,
																		match.credential.id,
																		request.envVar,
																	);
																}}
															>
																Use matching credential
															</Button>
														</div>
													{/each}
												</div>
											{/if}
										</div>

										<div class="space-y-2">
											<p
												class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
											>
												Select any credential
											</p>
											{#if anyCredentials.length === 0}
												<p class="text-muted-foreground text-sm">
													No additional credentials available.
												</p>
											{:else}
												<Select
													type="single"
													value={selectedAnyCredentialIdsByEnvVar[
														request.envVar
													] ?? ""}
													onValueChange={(value) => {
														selectedAnyCredentialIdsByEnvVar = {
															...selectedAnyCredentialIdsByEnvVar,
															[request.envVar]: value,
														};
													}}
												>
													<SelectTrigger class="w-full">
														{selectedAnyCredential
															? credentialDisplayName(selectedAnyCredential)
															: "Choose any credential"}
													</SelectTrigger>
													<SelectContent>
														{#each anyCredentials as match (match.credential.id)}
															<SelectItem value={match.credential.id}>
																{credentialDisplayName(match.credential)}
															</SelectItem>
														{/each}
													</SelectContent>
												</Select>
												{#if selectedAnyCredential}
													<p class="text-muted-foreground text-xs">
														{credentialBindingDescription(
															request.envVar,
															selectedAnyCredential,
														)}
													</p>
												{/if}
												<div class="flex justify-end">
													<Button
														variant="outline"
														size="sm"
														disabled={actionEnvVar === request.envVar ||
															!selectedAnyCredential}
														onclick={() => {
															void useSelectedAnyCredential(request);
														}}
													>
														Use selected credential
													</Button>
												</div>
											{/if}
										</div>
									{:else if selectedMode === "create"}
										<div
											class="space-y-2 rounded-md border border-dashed border-border p-3"
										>
											<p
												class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
											>
												Enter new one
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
												value={createCredentialSecretsByEnvVar[
													request.envVar
												] ?? ""}
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
											<div class="flex justify-end">
												<Button
													size="sm"
													disabled={actionEnvVar === request.envVar}
													onclick={() => {
														void createAndUseCredential(request);
													}}
												>
													Create and use
												</Button>
											</div>
										</div>
									{:else}
										<div
											class="space-y-2 rounded-md border border-dashed border-border p-3"
										>
											<p
												class="font-medium text-xs uppercase tracking-wide text-muted-foreground"
											>
												Reject request
											</p>
											<textarea
												class="min-h-20 w-full resize-y rounded-md border border-input bg-background px-3 py-2 text-sm outline-none placeholder:text-muted-foreground focus:border-primary focus:ring-1 focus:ring-primary/30"
												disabled={actionEnvVar === "__reject__"}
												oninput={(event) => {
													rejectionReason = (
														event.currentTarget as HTMLTextAreaElement
													).value;
												}}
												placeholder="Explain why you won't provide this credential..."
												value={rejectionReason}
											></textarea>
											<div class="flex justify-end">
												<Button
													variant="outline"
													disabled={actionEnvVar === "__reject__"}
													onclick={() => {
														void submitCredentialRejection(rejectionReason);
													}}
												>
													{actionEnvVar === "__reject__"
														? "Rejecting..."
														: "Reject request"}
												</Button>
											</div>
										</div>
									{/if}
								</div>
							{/if}
						</div>
					{/each}

					{#if allCredentialsResolved}
						<p class="text-muted-foreground text-sm">
							All requested credentials have been assigned to this session.
							Continuing…
						</p>
					{/if}
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
