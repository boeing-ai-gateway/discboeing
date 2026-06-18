<script lang="ts">
	import { onMount, tick } from "svelte";
	import type { BrowserEventChunkData, ChatMessage } from "$lib/api-types";
	import ConversationPane from "$lib/components/app/ConversationPane.svelte";
	import ConversationWebComponentRenderer from "$lib/components/app/parts/ConversationWebComponentRenderer.svelte";

	const sampleMessages = [
		{
			id: "sample-user-1",
			role: "user",
			metadata: {
				originalText: "Please inspect this repo and summarize what changed.",
				discobot: { turnId: "sample-turn-1" },
			},
			parts: [
				{
					type: "text",
					text: "Please inspect this repo and summarize what changed.\n\nInclude screenshots if browser activity happens.",
				},
				{
					type: "file",
					filename: "src/App.svelte",
					mediaType: "text/svelte",
					url: "file:///workspace/src/App.svelte",
				},
			],
		},
		{
			id: "sample-assistant-1",
			role: "assistant",
			metadata: {
				model: "gpt-5.5",
				reasoning: "medium",
				discobot: { turnId: "sample-turn-1" },
			},
			parts: [
				{
					type: "reasoning",
					state: "done",
					text: "I need to inspect the file tree, read the changed files, and then verify the UI visually.",
				},
				{
					type: "dynamic-tool",
					toolName: "Read",
					toolCallId: "tool-read-1",
					state: "output-available",
					title: "Read src/App.svelte",
					input: { file_path: "/workspace/src/App.svelte" },
					output: {
						content: [
							"<script>",
							"\tlet greeting = 'Hello Discobot';",
							"<" + "/script>",
							"",
							"<h1>{greeting}</h1>",
						].join("\n"),
					},
				},
				{
					type: "text",
					text: "## Summary\n\nThe app now renders a simple Svelte greeting.\n\n- Uses a local `greeting` value\n- Displays it in an `h1`\n- No server changes were required\n\n```svelte\n<h1>{greeting}</h1>\n```",
				},
			],
		},
		{
			id: "sample-user-2",
			role: "user",
			metadata: {
				discobot: { turnId: "sample-turn-2" },
			},
			parts: [
				{
					type: "text",
					text: "Run the tests. If a destructive action would be needed, ask first.",
				},
			],
		},
		{
			id: "sample-assistant-streaming",
			role: "assistant",
			status: "streaming",
			metadata: {
				model: "gpt-5.5",
				discobot: { turnId: "sample-turn-2" },
			},
			parts: [
				{
					type: "reasoning",
					state: "streaming",
					text: "I can run the safe test command and report the result.",
				},
				{
					type: "dynamic-tool",
					toolName: "Bash",
					toolCallId: "tool-test-1",
					state: "input-available",
					title: "Run UI tests",
					input: {
						command: "pnpm --dir ui test:ui",
						description: "Run Svelte UI tests",
					},
				},
				{
					type: "dynamic-tool",
					toolName: "Bash",
					toolCallId: "tool-denied-1",
					state: "approval-requested",
					title: "Remove build output",
					input: {
						command: "rm -rf ui/build",
						description: "Delete generated build output",
					},
				},
				{
					type: "dynamic-tool",
					toolName: "Bash",
					toolCallId: "tool-error-1",
					state: "output-error",
					title: "Run failing command",
					input: { command: "false" },
					output: "exit status 1",
				},
				{
					type: "text",
					text: "I started the safe checks and paused before doing anything destructive.",
				},
			],
		},
		{
			id: "sample-user-3",
			role: "user",
			metadata: {
				discobot: { turnId: "sample-turn-3" },
			},
			parts: [
				{
					type: "text",
					text: "Show me every optimized tool renderer so we can compare the web component defaults.",
				},
			],
		},
		{
			id: "sample-assistant-tool-gallery",
			role: "assistant",
			metadata: {
				model: "gpt-5.5",
				discobot: { turnId: "sample-turn-3" },
			},
			parts: [
				{
					type: "dynamic-tool",
					toolName: "AskUserQuestion",
					toolCallId: "tool-gallery-ask-user-question",
					state: "output-available",
					input: {
						questions: [
							{
								header: "Choose a renderer",
								question: "Which optimized renderer should we inspect first?",
								multiSelect: false,
								options: [
									{
										label: "Bash",
										description: "Inspect shell command rendering.",
									},
									{
										label: "TodoWrite",
										description: "Inspect task progress rendering.",
									},
								],
							},
						],
					},
					output: {
						"Which optimized renderer should we inspect first?": "Bash",
					},
				},
				{
					type: "dynamic-tool",
					toolName: "Bash",
					toolCallId: "tool-gallery-bash",
					state: "output-available",
					input: {
						command: "pnpm --dir ui check",
						description: "Run UI checks",
					},
					output: "svelte-check found 0 errors and 0 warnings",
				},
				{
					type: "dynamic-tool",
					toolName: "PowerShell",
					toolCallId: "tool-gallery-powershell",
					state: "output-available",
					input: {
						command: "Get-ChildItem ui",
						description: "List UI directory",
					},
					output: "Directory: /workspace/ui",
				},
				{
					type: "dynamic-tool",
					toolName: "Read",
					toolCallId: "tool-gallery-read",
					state: "output-available",
					input: { file_path: "/workspace/ui/src/App.svelte" },
					output: '<script>\n\tlet greeting = "Hello";\n<' + "/script>",
				},
				{
					type: "dynamic-tool",
					toolName: "read",
					toolCallId: "tool-gallery-lowercase-read",
					state: "output-available",
					input: { file_path: "/workspace/README.md" },
					output: "# Discobot\n\nA coding agent manager.",
				},
				{
					type: "dynamic-tool",
					toolName: "RequestCommitPull",
					toolCallId: "tool-gallery-request-commit-pull",
					state: "output-available",
					input: {
						baseCommit: "abc1234",
						notes: "Preview the prepared conversation renderer commit.",
					},
					output:
						"The user approved pulling the prepared sandbox commit into the host workspace.",
				},
				{
					type: "dynamic-tool",
					toolName: "Write",
					toolCallId: "tool-gallery-write",
					state: "output-available",
					input: {
						file_path: "/workspace/tmp/example.txt",
						content: "Hello from the write renderer.\n",
					},
					output: "Wrote /workspace/tmp/example.txt",
				},
				{
					type: "dynamic-tool",
					toolName: "Edit",
					toolCallId: "tool-gallery-edit",
					state: "output-available",
					input: {
						file_path: "/workspace/src/App.svelte",
						old_string: "Hello",
						new_string: "Hello Discobot",
					},
					output: "Updated /workspace/src/App.svelte",
				},
				{
					type: "dynamic-tool",
					toolName: "Grep",
					toolCallId: "tool-gallery-grep",
					state: "output-available",
					input: {
						pattern: "disco-conversation",
						path: "/workspace/ui/src",
						glob: "*.svelte",
						output_mode: "content",
					},
					output:
						"ui/src/lib/components/app/parts/ConversationWebComponentRenderer.svelte: <disco-conversation>",
				},
				{
					type: "dynamic-tool",
					toolName: "Glob",
					toolCallId: "tool-gallery-glob",
					state: "output-available",
					input: {
						path: "/workspace/ui/src/lib/web-components/conversation",
						pattern: "*.svelte",
					},
					output: [
						"DiscoConversation.svelte",
						"DiscoMessage.svelte",
						"DiscoToolCall.svelte",
					],
				},
				{
					type: "dynamic-tool",
					toolName: "RequestUserCredential",
					toolCallId: "tool-gallery-request-user-credential",
					state: "output-available",
					input: {
						credentials: [
							{
								name: "GitHub token",
								envVar: "GITHUB_TOKEN",
								justification: "Open a pull request for the prepared changes.",
								approvedUses: [
									{
										description: "Create pull requests with gh",
									},
								],
							},
						],
					},
					output:
						'__request_user_credential_granted__ [{"envVar":"GITHUB_TOKEN"}]',
				},
				{
					type: "dynamic-tool",
					toolName: "apply_patch",
					toolCallId: "tool-gallery-apply-patch",
					state: "output-available",
					input:
						"*** Begin Patch\n*** Update File: src/App.svelte\n@@\n-Hello\n+Hello Discobot\n*** End Patch",
					output: "Success. Updated the following files:\nM src/App.svelte",
				},
				{
					type: "dynamic-tool",
					toolName: "WebSearch",
					toolCallId: "tool-gallery-web-search",
					state: "output-available",
					input: {
						query: "Discobot web component renderer",
						type: "search",
						allowed_domains: ["example.com"],
					},
					output: {
						results: [
							{
								title: "Discobot renderer notes",
								url: "https://example.com/discobot-renderer",
								snippet: "A summary of the renderer comparison work.",
							},
						],
					},
				},
				{
					type: "dynamic-tool",
					toolName: "WebFetch",
					toolCallId: "tool-gallery-web-fetch",
					state: "output-available",
					input: {
						url: "https://example.com/discobot-renderer",
						prompt: "Summarize the renderer notes.",
					},
					output: {
						content:
							"The renderer notes describe a DOM-first conversation component.",
					},
				},
				{
					type: "dynamic-tool",
					toolName: "TodoWrite",
					toolCallId: "tool-gallery-todo-write",
					state: "output-available",
					input: {
						todos: [
							{
								content: "Add step divider parity",
								activeForm: "Adding step divider parity",
								status: "completed",
							},
							{
								content: "Port optimized tool renderers",
								activeForm: "Porting optimized tool renderers",
								status: "in_progress",
							},
							{
								content: "Review browser activity details",
								activeForm: "Reviewing browser activity details",
								status: "pending",
							},
						],
					},
					output: { success: true, content: "Updated todo list" },
				},
				{
					type: "dynamic-tool",
					toolName: "Task",
					toolCallId: "tool-gallery-task",
					state: "output-available",
					input: {
						description: "Audit optimized renderers",
						prompt:
							"Check that every optimized renderer appears in the comparison page.",
						subagent_type: "general-purpose",
					},
					output: "All optimized renderers are represented in the sample data.",
				},
				{
					type: "dynamic-tool",
					toolName: "Skill",
					toolCallId: "tool-gallery-skill",
					state: "output-available",
					input: {
						skill: "commit",
						args: "conversation renderer parity",
					},
					output: "Prepared a commit summary.",
				},
				{
					type: "text",
					text: "All registered optimized tool renderers are represented above.",
				},
			],
		},
		{
			id: "sample-user-generated-attachment",
			role: "user",
			metadata: {
				originalText: "Summarize the attached renderer notes.",
				discobot: { turnId: "sample-turn-4" },
			},
			parts: [
				{
					type: "text",
					text: "Summarize the attached renderer notes and call out the generated text path, attachment part rendering, and visual differences between current and web-component conversations.",
				},
				{
					type: "file",
					filename: "renderer-notes.md",
					mediaType: "text/markdown",
					url: "file:///workspace/docs/renderer-notes.md",
				},
			],
		},
		{
			id: "sample-assistant-generated-attachment",
			role: "assistant",
			metadata: {
				model: "gpt-5.5",
				discobot: { turnId: "sample-turn-4" },
			},
			parts: [
				{
					type: "reasoning",
					state: "done",
					text: "I should account for the original prompt, generated text, and attached notes as parts of one user message.",
				},
				{
					type: "text",
					text: "The user message now keeps original text, generated text, and the attachment together inside one message bubble.",
				},
			],
		},
		{
			id: "sample-user-skill-command",
			role: "user",
			metadata: {
				originalText: "/skill browser Inspect the comparison page",
				slashCommand: {
					kind: "skill",
					name: "browser",
					text: "Open the conversation comparison page, inspect the generated text and attachment parts, and report visual parity issues.",
				},
				discobot: { turnId: "sample-turn-5" },
			},
			parts: [
				{
					type: "text",
					text: "Open the conversation comparison page, inspect the generated text and attachment parts, and report visual parity issues.",
				},
			],
		},
		{
			id: "sample-assistant-skill-command",
			role: "assistant",
			metadata: {
				model: "gpt-5.5",
				discobot: { turnId: "sample-turn-5" },
			},
			parts: [
				{
					type: "text",
					text: "I would invoke the browser skill with the generated skill text shown inside the user message.",
				},
			],
		},
		{
			id: "sample-user-script-command",
			role: "user",
			metadata: {
				originalText: "/script renderer-smoke",
				slashCommand: {
					kind: "script",
					name: "renderer-smoke",
					text: "The script generated a renderer smoke-test prompt from workspace state.",
					script: {
						scriptName: "renderer-smoke",
						scriptPath: ".discobot/scripts/renderer-smoke.sh",
						exitCode: 0,
						success: true,
						stdout: "Generated renderer smoke-test prompt.",
					},
				},
				discobot: { turnId: "sample-turn-6" },
			},
			parts: [
				{
					type: "text",
					text: "The script generated a renderer smoke-test prompt from workspace state.",
				},
			],
		},
		{
			id: "sample-assistant-script-command",
			role: "assistant",
			metadata: {
				model: "gpt-5.5",
				discobot: { turnId: "sample-turn-6" },
			},
			parts: [
				{
					type: "text",
					text: "The script command generated prompt text, and the assistant response remains the trailing content part for the turn.",
				},
			],
		},
		{
			id: "sample-user-command-command",
			role: "user",
			metadata: {
				originalText: "/command pnpm --dir ui check",
				slashCommand: {
					kind: "command",
					name: "command",
				},
				discobot: { turnId: "sample-turn-7" },
			},
			parts: [
				{
					type: "text",
					text: "Run `pnpm --dir ui check` and summarize any renderer regressions found by format, lint, or typecheck.",
				},
			],
		},
		{
			id: "sample-assistant-command-command",
			role: "assistant",
			metadata: {
				model: "gpt-5.5",
				discobot: { turnId: "sample-turn-7" },
			},
			parts: [
				{
					type: "text",
					text: "The command request is represented as one user message followed by this assistant content message.",
				},
			],
		},
		{
			id: "sample-user-ask-question",
			role: "user",
			metadata: {
				discobot: { turnId: "sample-turn-8" },
			},
			parts: [
				{
					type: "text",
					text: "Ask me which renderer paths and states to inspect before making more changes.",
				},
			],
		},
		{
			id: "sample-assistant-ask-question",
			role: "assistant",
			status: "streaming",
			metadata: {
				model: "gpt-5.5",
				discobot: { turnId: "sample-turn-8" },
			},
			parts: [
				{
					type: "reasoning",
					state: "done",
					text: "I should pause for the user's choice before continuing the renderer audit.",
				},
				{
					type: "dynamic-tool",
					toolName: "AskUserQuestion",
					toolCallId: "tool-pending-ask-user-question",
					state: "approval-requested",
					title: "Choose renderer inspection targets",
					approval: { id: "approval-pending-ask-user-question" },
					input: {
						questions: [
							{
								header: "Primary renderer",
								question: "Which renderer should we inspect first?",
								notes:
									"The comparison page should exercise the same pending question picker that appears during a paused live turn.",
								multiSelect: false,
								options: [
									{
										label: "Current ConversationPane",
										description:
											"Start with the existing Svelte renderer on the left.",
									},
									{
										label: "Web components, explicit turns",
										description:
											"Start with the custom-element turn renderer in the middle.",
									},
									{
										label: "Web components, flat messages",
										description:
											"Start with the flat custom-element renderer on the right.",
									},
								],
							},
							{
								header: "States to verify",
								question: "Which question states should the sample cover?",
								multiSelect: true,
								options: [
									{
										label: "Single select",
										description:
											"Verify one choice auto-advances to the next step.",
									},
									{
										label: "Multi select",
										description: "Verify multiple options can be toggled.",
									},
									{
										label: "Other",
										description: "Verify custom answer text is preserved.",
									},
								],
							},
						],
					},
				},
			],
		},
	] as ChatMessage[];

	const sampleBrowserEventsByTurnId: Record<string, BrowserEventChunkData[]> = {
		"sample-turn-1": [
			{
				threadId: "sample-thread",
				turnId: "sample-turn-1",
				assistantMessageId: "sample-assistant-1",
				stepIndex: 1,
				event: {
					eventId: "browser-event-1",
					stepIndex: 1,
					method: "browser_navigate",
					direction: "request",
					payload: { url: "http://localhost:3100" },
					recordedAt: "2026-06-15T17:44:38Z",
				},
			},
			{
				threadId: "sample-thread",
				turnId: "sample-turn-1",
				assistantMessageId: "sample-assistant-1",
				stepIndex: 2,
				event: {
					eventId: "browser-event-2",
					stepIndex: 2,
					method: "browser_screenshot",
					direction: "response",
					payload: { title: "Discobot UI" },
					files: [
						{
							path: "/tmp/browser-step-2.png",
							uri: "artifacts:///tmp/browser-step-2.png",
							filename: "browser-step-2.png",
							mediaType: "image/png",
						},
					],
					recordedAt: "2026-06-15T17:44:40Z",
				},
			},
		],
	};

	let explicitSection: HTMLElement | undefined = $state();
	let rawSection: HTMLElement | undefined = $state();
	let comparisonTheme: "light" | "dark" = $state("light");

	function toggleComparisonTheme() {
		comparisonTheme = comparisonTheme === "dark" ? "light" : "dark";
	}

	onMount(() => {
		void tick().then(() => {
			explicitSection?.scrollTo({ top: explicitSection.scrollHeight });
			rawSection?.scrollTo({ top: rawSection.scrollHeight });
		});
	});
</script>

<svelte:head>
	<title>Conversation renderer comparison</title>
</svelte:head>

<div
	class="flex h-screen min-h-0 min-w-[1800px] flex-col bg-background text-foreground"
	class:dark={comparisonTheme === "dark"}
	style:color-scheme={comparisonTheme}
>
	<header
		class="shrink-0 border-b border-border bg-background/95 px-4 py-3 backdrop-blur"
	>
		<div class="flex flex-wrap items-center justify-between gap-3 text-sm">
			<div class="flex flex-wrap items-center gap-x-4 gap-y-1">
				<h1 class="text-base font-semibold">
					Conversation renderer comparison
				</h1>
				<span class="text-muted-foreground">sample messages</span>
				<span class="text-muted-foreground"
					>{sampleMessages.length} messages</span
				>
			</div>
			<button
				type="button"
				class="inline-flex items-center gap-2 rounded-md border border-border bg-background px-3 py-1.5 font-medium text-xs shadow-xs transition-colors hover:bg-accent hover:text-accent-foreground"
				aria-pressed={comparisonTheme === "dark"}
				onclick={toggleComparisonTheme}
			>
				<span class="text-muted-foreground">Theme</span>
				<span>{comparisonTheme === "dark" ? "Dark" : "Light"}</span>
			</button>
		</div>
		<p class="mt-1 text-muted-foreground text-xs">
			Left: current renderer. Middle: custom elements with explicit turns and
			browser activity. Right: custom elements as flat messages with the same
			optimized tool renderers.
		</p>
	</header>

	<div
		class="grid min-h-0 flex-1 min-w-[1800px] grid-cols-3 divide-x divide-border"
	>
		<section class="flex min-h-0 flex-col">
			<h2
				class="shrink-0 border-b border-border bg-background/95 px-4 py-2 text-sm font-semibold"
			>
				Current ConversationPane
			</h2>
			<div class="min-h-0 flex-1">
				<ConversationPane
					messages={sampleMessages}
					browserEventsByTurnId={sampleBrowserEventsByTurnId}
					status="ready"
					showComposer={false}
					toolDefaultOpen
				/>
			</div>
		</section>

		<section class="min-h-0 overflow-auto p-4" bind:this={explicitSection}>
			<h2 class="mb-4 text-sm font-semibold">Web components, explicit turns</h2>
			<ConversationWebComponentRenderer
				messages={sampleMessages}
				browserEventsByTurnId={sampleBrowserEventsByTurnId}
				status="ready"
				chatWidth="constrained"
				resolvedTheme={comparisonTheme}
				renderTurns
			/>
		</section>

		<section class="min-h-0 overflow-auto p-4" bind:this={rawSection}>
			<h2 class="mb-4 text-sm font-semibold">Web components, flat messages</h2>
			<ConversationWebComponentRenderer
				messages={sampleMessages}
				status="ready"
				chatWidth="constrained"
				resolvedTheme={comparisonTheme}
			/>
		</section>
	</div>
</div>
