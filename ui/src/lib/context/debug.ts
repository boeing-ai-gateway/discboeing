import { untrack } from "svelte";

import { readStorage, writeStorage } from "$lib/local-storage";
import type {
	ProjectSocketMessage,
	ProjectSocketRequest,
} from "$lib/context/project-subscription";

const MAX_DEBUG_LOG_ENTRIES = 300;
const MAX_DEBUG_REQUEST_ENTRIES = 200;
const MAX_DEBUG_EVENT_ENTRIES = 500;
const MAX_DEBUG_STATE_CHANGE_ENTRIES = 100;
const MAX_DEBUG_STATE_DIFFS_PER_ENTRY = 80;
const MAX_SERIALIZED_LENGTH = 600;
const MAX_COMMAND_CALL_STACK_LENGTH = 8_000;
const MAX_DEBUG_STORAGE_LENGTH = 2_000_000;
const DEBUG_STORAGE_KEY = "discobot.ng.debug";

let nextDebugId = 1;
let persistTimer: number | undefined;

export type DebugLogKind =
	| "command"
	| "subscription"
	| "event"
	| "console"
	| "network";
export type DebugLogLevel = "info" | "warn" | "error";
export type DebugSubscriptionStatus = "opening" | "active" | "closed" | "error";

export type DebugLogEntry = {
	id: number;
	at: string;
	kind: DebugLogKind;
	level: DebugLogLevel;
	message: string;
	detail?: string;
};

export type DebugSubscriptionEntry = {
	id: string;
	stream: string;
	label: string;
	sessionId?: string;
	threadId?: string;
	serviceId?: string;
	status: DebugSubscriptionStatus;
	openedAt: string;
	closedAt: string | null;
	lastEventAt: string | null;
	eventCount: number;
	lastEvent: string | null;
};

export type DebugCommandEntry = {
	id: number;
	name: string;
	status: "running" | "success" | "error";
	startedAt: string;
	finishedAt: string | null;
	durationMs: number | null;
	args: string;
	callStack: string | null;
	error: string | null;
};

export type DebugRequestEntry = {
	id: number;
	method: string;
	url: string;
	status: "pending" | "success" | "error";
	statusCode: number | null;
	statusText: string | null;
	startedAt: string;
	finishedAt: string | null;
	durationMs: number | null;
	error: string | null;
};

export type DebugEventEntry = {
	id: number;
	at: string;
	direction: "in" | "out";
	stream: string;
	type: string;
	event: string | null;
	label: string;
	payload: string;
};

export type DebugStateDiffEntry = {
	path: string;
	type: "added" | "changed" | "removed";
	before: string;
	after: string;
};

export type DebugStateChangeEntry = {
	id: number;
	at: string;
	changeCount: number;
	changes: DebugStateDiffEntry[];
};

export type DebugState = {
	enabled: boolean;
	commands: DebugCommandEntry[];
	events: DebugEventEntry[];
	logs: DebugLogEntry[];
	requests: DebugRequestEntry[];
	stateChanges: DebugStateChangeEntry[];
	subscriptions: Record<string, DebugSubscriptionEntry>;
};

type DebugContext = {
	view: {
		app: {
			debug: DebugState;
		};
	};
	data: unknown;
};

type ConsoleMethod = "debug" | "error" | "info" | "log" | "warn";

export function createDebugState(): DebugState {
	const enabled = import.meta.env.DEV;
	const persisted = enabled ? readPersistedDebugState() : null;
	const debugState = persisted ?? {
		enabled: import.meta.env.DEV,
		commands: [],
		events: [],
		logs: [],
		requests: [],
		stateChanges: [],
		subscriptions: {},
	};
	debugState.enabled = enabled;
	setNextDebugId(debugState);
	return debugState;
}

export function installDebugInstrumentation(context: DebugContext): () => void {
	if (!context.view.app.debug.enabled || typeof window === "undefined") {
		return () => {};
	}

	const originalConsole = {
		debug: console.debug,
		error: console.error,
		info: console.info,
		log: console.log,
		warn: console.warn,
	} satisfies Record<ConsoleMethod, typeof console.log>;
	const originalFetch = globalThis.fetch;
	let previousStateSnapshot = stringifyStateSnapshot(context);
	const stateInterval = window.setInterval(() => {
		logDebugStateChange(context, previousStateSnapshot);
		previousStateSnapshot = stringifyStateSnapshot(context);
	}, 1000);

	const consoleMethods: ConsoleMethod[] = [
		"debug",
		"error",
		"info",
		"log",
		"warn",
	];
	for (const method of consoleMethods) {
		console[method] = (...args: unknown[]) => {
			originalConsole[method](...args);
			logDebugConsole(context, method, args);
		};
	}

	const handleError = (event: ErrorEvent) => {
		pushDebugLog(
			context,
			"console",
			"error",
			"Uncaught error",
			formatErrorWithStack(event.error ?? event.message),
		);
	};
	const handleUnhandledRejection = (event: PromiseRejectionEvent) => {
		pushDebugLog(
			context,
			"console",
			"error",
			"Unhandled promise rejection",
			formatErrorWithStack(event.reason),
		);
	};
	const handleBeforeUnload = () => {
		persistDebugState(context);
	};

	globalThis.fetch = (async (
		input: RequestInfo | URL,
		init?: RequestInit,
	): Promise<Response> => {
		const requestId = logDebugRequestStart(context, input, init);
		try {
			const response = await originalFetch(input, init);
			logDebugRequestFinish(context, requestId, response);
			return response;
		} catch (error) {
			logDebugRequestFinish(context, requestId, undefined, error);
			throw error;
		}
	}) as typeof globalThis.fetch;

	window.addEventListener("error", handleError);
	window.addEventListener("unhandledrejection", handleUnhandledRejection);
	window.addEventListener("beforeunload", handleBeforeUnload);

	return () => {
		for (const method of consoleMethods) {
			console[method] = originalConsole[method];
		}
		globalThis.fetch = originalFetch;
		window.clearInterval(stateInterval);
		persistDebugState(context);
		window.removeEventListener("error", handleError);
		window.removeEventListener("unhandledrejection", handleUnhandledRejection);
		window.removeEventListener("beforeunload", handleBeforeUnload);
	};
}

export function logDebugCommandStart(
	context: DebugContext,
	name: string,
	args: unknown[],
): number | null {
	return untrack(() => {
		if (!context.view.app.debug.enabled) return null;
		const id = nextId();
		const startedAt = now();
		context.view.app.debug.commands = trimEntries([
			{
				id,
				name,
				status: "running",
				startedAt,
				finishedAt: null,
				durationMs: null,
				args: serialize(args),
				callStack: captureCommandCallStack(),
				error: null,
			},
			...context.view.app.debug.commands,
		]);
		pushDebugLog(context, "command", "info", `command ${name} started`);
		scheduleDebugPersist(context);
		return id;
	});
}

export function logDebugCommandFinish(
	context: DebugContext,
	id: number | null,
	status: "success" | "error",
	error?: unknown,
): void {
	untrack(() => {
		if (!context.view.app.debug.enabled || id === null) return;
		const finishedAt = now();
		const command = context.view.app.debug.commands.find(
			(entry) => entry.id === id,
		);
		if (!command) return;
		command.status = status;
		command.finishedAt = finishedAt;
		command.durationMs = Date.parse(finishedAt) - Date.parse(command.startedAt);
		command.error = error ? formatError(error) : null;
		pushDebugLog(
			context,
			"command",
			status === "error" ? "error" : "info",
			`command ${command.name} ${status}`,
			command.error ?? undefined,
		);
		scheduleDebugPersist(context);
	});
}

export function openDebugSubscription(
	context: DebugContext,
	request: ProjectSocketRequest,
): void {
	untrack(() => {
		if (!context.view.app.debug.enabled) return;
		const id = subscriptionId(request);
		const openedAt = now();
		context.view.app.debug.subscriptions[id] = {
			id,
			stream: request.stream,
			label: subscriptionLabel(request),
			sessionId: "sessionId" in request ? request.sessionId : undefined,
			threadId: "threadId" in request ? request.threadId : undefined,
			serviceId: "serviceId" in request ? request.serviceId : undefined,
			status: "opening",
			openedAt,
			closedAt: null,
			lastEventAt: null,
			eventCount: 0,
			lastEvent: null,
		};
		pushDebugLog(
			context,
			"subscription",
			"info",
			`opened ${subscriptionLabel(request)}`,
		);
		scheduleDebugPersist(context);
	});
}

export function activateDebugSubscription(
	context: DebugContext,
	request: ProjectSocketRequest,
): void {
	untrack(() => {
		if (!context.view.app.debug.enabled) return;
		const subscription =
			context.view.app.debug.subscriptions[subscriptionId(request)];
		if (subscription) subscription.status = "active";
		scheduleDebugPersist(context);
	});
}

export function closeDebugSubscription(
	context: DebugContext,
	request: ProjectSocketRequest,
	error?: unknown,
): void {
	untrack(() => {
		if (!context.view.app.debug.enabled) return;
		const subscription =
			context.view.app.debug.subscriptions[subscriptionId(request)];
		if (!subscription) return;
		subscription.status = error ? "error" : "closed";
		subscription.closedAt = now();
		pushDebugLog(
			context,
			"subscription",
			error ? "error" : "info",
			`${error ? "errored" : "closed"} ${subscription.label}`,
			error ? formatError(error) : undefined,
		);
		scheduleDebugPersist(context);
	});
}

export function logDebugSubscriptionEvent(
	context: DebugContext,
	request: ProjectSocketRequest,
	event: string,
	detail?: unknown,
): void {
	untrack(() => {
		if (!context.view.app.debug.enabled) return;
		const subscription =
			context.view.app.debug.subscriptions[subscriptionId(request)];
		if (subscription) {
			subscription.status = "active";
			subscription.eventCount += 1;
			subscription.lastEventAt = now();
			subscription.lastEvent = event;
		}
		pushDebugLog(
			context,
			"event",
			"info",
			`${subscription?.label ?? subscriptionLabel(request)} event ${event}`,
			detail === undefined ? undefined : serialize(detail),
		);
		scheduleDebugPersist(context);
	});
}

export function logDebugSocketMessage(
	context: DebugContext,
	direction: "in" | "out",
	message: ProjectSocketMessage | ProjectSocketRequest,
): void {
	untrack(() => {
		if (!context.view.app.debug.enabled) return;
		const stream =
			"stream" in message && message.stream ? message.stream : "unknown";
		const type = "type" in message && message.type ? message.type : "unknown";
		const event =
			"event" in message && typeof message.event === "string"
				? message.event
				: null;
		context.view.app.debug.events = trimEventEntries([
			{
				id: nextId(),
				at: now(),
				direction,
				stream,
				type,
				event,
				label: socketMessageLabel(direction, message),
				payload: serialize(message),
			},
			...context.view.app.debug.events,
		]);
		scheduleDebugPersist(context);
	});
}

export function clearDebugLogs(context: DebugContext): void {
	untrack(() => {
		context.view.app.debug.logs = [];
		context.view.app.debug.commands = [];
		context.view.app.debug.events = [];
		context.view.app.debug.requests = [];
		context.view.app.debug.stateChanges = [];
		context.view.app.debug.subscriptions = Object.fromEntries(
			Object.entries(context.view.app.debug.subscriptions).filter(
				([, subscription]) =>
					subscription.status === "active" || subscription.status === "opening",
			),
		);
		persistDebugState(context);
	});
}

function logDebugStateChange(
	context: DebugContext,
	previousSnapshot: string,
): void {
	untrack(() => {
		if (!context.view.app.debug.enabled) return;
		const nextSnapshot = stringifyStateSnapshot(context);
		if (nextSnapshot === previousSnapshot) return;

		try {
			const before = JSON.parse(previousSnapshot) as unknown;
			const after = JSON.parse(nextSnapshot) as unknown;
			const changes = diffJsonValues(before, after);
			if (changes.length === 0) return;
			context.view.app.debug.stateChanges = trimStateChangeEntries([
				{
					id: nextId(),
					at: now(),
					changeCount: changes.length,
					changes,
				},
				...context.view.app.debug.stateChanges,
			]);
			scheduleDebugPersist(context);
		} catch (error) {
			pushDebugLog(
				context,
				"console",
				"error",
				"State diff failed",
				formatError(error),
			);
		}
	});
}

function logDebugConsole(
	context: DebugContext,
	method: ConsoleMethod,
	args: unknown[],
): void {
	untrack(() => {
		if (!context.view.app.debug.enabled) return;
		pushDebugLog(
			context,
			"console",
			method === "error" ? "error" : method === "warn" ? "warn" : "info",
			`console.${method}`,
			args.map(formatConsoleArg).join("\n"),
		);
		scheduleDebugPersist(context);
	});
}

function logDebugRequestStart(
	context: DebugContext,
	input: RequestInfo | URL,
	init?: RequestInit,
): number | null {
	return untrack(() => {
		if (!context.view.app.debug.enabled) return null;
		const id = nextId();
		context.view.app.debug.requests = trimRequestEntries([
			{
				id,
				method: requestMethod(input, init),
				url: requestUrl(input),
				status: "pending",
				statusCode: null,
				statusText: null,
				startedAt: now(),
				finishedAt: null,
				durationMs: null,
				error: null,
			},
			...context.view.app.debug.requests,
		]);
		scheduleDebugPersist(context);
		return id;
	});
}

function logDebugRequestFinish(
	context: DebugContext,
	id: number | null,
	response?: Response,
	error?: unknown,
): void {
	untrack(() => {
		if (!context.view.app.debug.enabled || id === null) return;
		const request = context.view.app.debug.requests.find(
			(entry) => entry.id === id,
		);
		if (!request) return;
		const finishedAt = now();
		request.finishedAt = finishedAt;
		request.durationMs = Date.parse(finishedAt) - Date.parse(request.startedAt);
		request.statusCode = response?.status ?? null;
		request.statusText = response?.statusText ?? null;
		request.error = error ? formatError(error) : null;
		request.status = error || (response && !response.ok) ? "error" : "success";
		pushDebugLog(
			context,
			"network",
			request.status === "error" ? "error" : "info",
			`${request.method} ${request.url} ${request.statusCode ?? request.status}`,
			request.error ?? request.statusText ?? undefined,
		);
		scheduleDebugPersist(context);
	});
}

function pushDebugLog(
	context: DebugContext,
	kind: DebugLogKind,
	level: DebugLogLevel,
	message: string,
	detail?: string,
): void {
	context.view.app.debug.logs = trimEntries([
		{
			id: nextId(),
			at: now(),
			kind,
			level,
			message,
			detail,
		},
		...context.view.app.debug.logs,
	]);
	scheduleDebugPersist(context);
}

function scheduleDebugPersist(context: DebugContext): void {
	if (typeof window === "undefined") return;
	if (persistTimer) return;
	persistTimer = window.setTimeout(() => {
		persistTimer = undefined;
		persistDebugState(context);
	}, 250);
}

function persistDebugState(context: DebugContext): void {
	if (persistTimer) {
		clearTimeout(persistTimer);
		persistTimer = undefined;
	}
	try {
		writeStorage(
			DEBUG_STORAGE_KEY,
			serializeDebugStateForStorage(context.view.app.debug),
		);
	} catch {
		// Best effort only: debugging history should never interrupt the app.
	}
}

function serializeDebugStateForStorage(debugState: DebugState): string | null {
	const persisted = cloneDebugState(debugState);
	let serialized = serializeDebugState(persisted);
	while (
		serialized &&
		serialized.length > MAX_DEBUG_STORAGE_LENGTH &&
		dropOldestPersistedEntry(persisted)
	) {
		serialized = serializeDebugState(persisted);
	}
	return serialized && serialized.length <= MAX_DEBUG_STORAGE_LENGTH
		? serialized
		: null;
}

function serializeDebugState(debugState: DebugState): string | null {
	try {
		return JSON.stringify(debugState);
	} catch {
		return null;
	}
}

function cloneDebugState(debugState: DebugState): DebugState {
	return {
		enabled: debugState.enabled,
		commands: [...debugState.commands],
		events: [...debugState.events],
		logs: [...debugState.logs],
		requests: [...debugState.requests],
		stateChanges: [...debugState.stateChanges],
		subscriptions: { ...debugState.subscriptions },
	};
}

function dropOldestPersistedEntry(debugState: DebugState): boolean {
	if (debugState.stateChanges.length > 0) {
		debugState.stateChanges.pop();
		return true;
	}
	if (debugState.events.length > 0) {
		debugState.events.pop();
		return true;
	}
	if (debugState.logs.length > 0) {
		debugState.logs.pop();
		return true;
	}
	if (debugState.requests.length > 0) {
		debugState.requests.pop();
		return true;
	}
	if (debugState.commands.length > 0) {
		debugState.commands.pop();
		return true;
	}
	return false;
}

function readPersistedDebugState(): DebugState | null {
	let stored: string | null;
	try {
		stored = readStorage(DEBUG_STORAGE_KEY);
	} catch {
		return null;
	}
	if (!stored) return null;
	try {
		return normalizeDebugState(JSON.parse(stored) as unknown);
	} catch {
		try {
			writeStorage(DEBUG_STORAGE_KEY, null);
		} catch {
			// Ignore storage cleanup failures.
		}
		return null;
	}
}

function normalizeDebugState(value: unknown): DebugState | null {
	if (!isRecord(value)) return null;
	return {
		enabled: import.meta.env.DEV,
		commands: Array.isArray(value.commands)
			? trimEntries(
					value.commands
						.filter(isDebugCommandEntry)
						.map(normalizePersistedCommand),
				)
			: [],
		events: Array.isArray(value.events)
			? trimEventEntries(value.events.filter(isDebugEventEntry))
			: [],
		logs: Array.isArray(value.logs)
			? trimEntries(value.logs.filter(isDebugLogEntry))
			: [],
		requests: Array.isArray(value.requests)
			? trimRequestEntries(value.requests.filter(isDebugRequestEntry))
			: [],
		stateChanges: Array.isArray(value.stateChanges)
			? trimStateChangeEntries(
					value.stateChanges.filter(isDebugStateChangeEntry),
				)
			: [],
		subscriptions: isRecord(value.subscriptions)
			? Object.fromEntries(
					Object.entries(value.subscriptions)
						.filter((entry): entry is [string, DebugSubscriptionEntry] =>
							isDebugSubscriptionEntry(entry[1]),
						)
						.map(([id, subscription]) => [
							id,
							normalizePersistedSubscription(subscription),
						]),
				)
			: {},
	};
}

function normalizePersistedSubscription(
	subscription: DebugSubscriptionEntry,
): DebugSubscriptionEntry {
	if (subscription.status !== "opening" && subscription.status !== "active") {
		return subscription;
	}
	return {
		...subscription,
		status: "closed",
		closedAt: subscription.closedAt ?? now(),
	};
}

function normalizePersistedCommand(
	command: DebugCommandEntry,
): DebugCommandEntry {
	return {
		...command,
		callStack: command.callStack ?? null,
	};
}

function setNextDebugId(debugState: DebugState): void {
	let maxId = 0;
	for (const entry of debugState.commands) maxId = Math.max(maxId, entry.id);
	for (const entry of debugState.events) maxId = Math.max(maxId, entry.id);
	for (const entry of debugState.logs) maxId = Math.max(maxId, entry.id);
	for (const entry of debugState.requests) maxId = Math.max(maxId, entry.id);
	for (const entry of debugState.stateChanges)
		maxId = Math.max(maxId, entry.id);
	nextDebugId = Math.max(nextDebugId, maxId + 1);
}

function isDebugLogEntry(value: unknown): value is DebugLogEntry {
	return (
		isRecord(value) &&
		typeof value.id === "number" &&
		typeof value.at === "string" &&
		isDebugLogKind(value.kind) &&
		isDebugLogLevel(value.level) &&
		typeof value.message === "string" &&
		(value.detail === undefined || typeof value.detail === "string")
	);
}

function isDebugSubscriptionEntry(
	value: unknown,
): value is DebugSubscriptionEntry {
	return (
		isRecord(value) &&
		typeof value.id === "string" &&
		typeof value.stream === "string" &&
		typeof value.label === "string" &&
		isDebugSubscriptionStatus(value.status) &&
		typeof value.openedAt === "string" &&
		(value.closedAt === null || typeof value.closedAt === "string") &&
		(value.lastEventAt === null || typeof value.lastEventAt === "string") &&
		typeof value.eventCount === "number" &&
		(value.lastEvent === null || typeof value.lastEvent === "string")
	);
}

function isDebugCommandEntry(value: unknown): value is DebugCommandEntry {
	return (
		isRecord(value) &&
		typeof value.id === "number" &&
		typeof value.name === "string" &&
		(value.status === "running" ||
			value.status === "success" ||
			value.status === "error") &&
		typeof value.startedAt === "string" &&
		(value.finishedAt === null || typeof value.finishedAt === "string") &&
		(value.durationMs === null || typeof value.durationMs === "number") &&
		typeof value.args === "string" &&
		(value.callStack === undefined ||
			value.callStack === null ||
			typeof value.callStack === "string") &&
		(value.error === null || typeof value.error === "string")
	);
}

function isDebugRequestEntry(value: unknown): value is DebugRequestEntry {
	return (
		isRecord(value) &&
		typeof value.id === "number" &&
		typeof value.method === "string" &&
		typeof value.url === "string" &&
		(value.status === "pending" ||
			value.status === "success" ||
			value.status === "error") &&
		(value.statusCode === null || typeof value.statusCode === "number") &&
		(value.statusText === null || typeof value.statusText === "string") &&
		typeof value.startedAt === "string" &&
		(value.finishedAt === null || typeof value.finishedAt === "string") &&
		(value.durationMs === null || typeof value.durationMs === "number") &&
		(value.error === null || typeof value.error === "string")
	);
}

function isDebugEventEntry(value: unknown): value is DebugEventEntry {
	return (
		isRecord(value) &&
		typeof value.id === "number" &&
		typeof value.at === "string" &&
		(value.direction === "in" || value.direction === "out") &&
		typeof value.stream === "string" &&
		typeof value.type === "string" &&
		(value.event === null || typeof value.event === "string") &&
		typeof value.label === "string" &&
		typeof value.payload === "string"
	);
}

function isDebugStateChangeEntry(
	value: unknown,
): value is DebugStateChangeEntry {
	return (
		isRecord(value) &&
		typeof value.id === "number" &&
		typeof value.at === "string" &&
		typeof value.changeCount === "number" &&
		Array.isArray(value.changes) &&
		value.changes.every(isDebugStateDiffEntry)
	);
}

function isDebugStateDiffEntry(value: unknown): value is DebugStateDiffEntry {
	return (
		isRecord(value) &&
		typeof value.path === "string" &&
		(value.type === "added" ||
			value.type === "changed" ||
			value.type === "removed") &&
		typeof value.before === "string" &&
		typeof value.after === "string"
	);
}

function isDebugLogKind(value: unknown): value is DebugLogKind {
	return (
		value === "command" ||
		value === "subscription" ||
		value === "event" ||
		value === "console" ||
		value === "network"
	);
}

function isDebugLogLevel(value: unknown): value is DebugLogLevel {
	return value === "info" || value === "warn" || value === "error";
}

function isDebugSubscriptionStatus(
	value: unknown,
): value is DebugSubscriptionStatus {
	return (
		value === "opening" ||
		value === "active" ||
		value === "closed" ||
		value === "error"
	);
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function trimEntries<T>(entries: T[]): T[] {
	return entries.slice(0, MAX_DEBUG_LOG_ENTRIES);
}

function trimRequestEntries<T>(entries: T[]): T[] {
	return entries.slice(0, MAX_DEBUG_REQUEST_ENTRIES);
}

function trimEventEntries<T>(entries: T[]): T[] {
	return entries.slice(0, MAX_DEBUG_EVENT_ENTRIES);
}

function trimStateChangeEntries<T>(entries: T[]): T[] {
	return entries.slice(0, MAX_DEBUG_STATE_CHANGE_ENTRIES);
}

function subscriptionId(request: ProjectSocketRequest): string {
	return [
		request.stream,
		"sessionId" in request ? request.sessionId : "",
		"threadId" in request ? request.threadId : "",
		"serviceId" in request ? request.serviceId : "",
	]
		.filter(Boolean)
		.join(":");
}

function subscriptionLabel(request: ProjectSocketRequest): string {
	const parts: string[] = [request.stream];
	if ("sessionId" in request && request.sessionId)
		parts.push(request.sessionId);
	if ("threadId" in request && request.threadId) parts.push(request.threadId);
	if ("serviceId" in request && request.serviceId)
		parts.push(request.serviceId);
	return parts.join(" / ");
}

function socketMessageLabel(
	direction: "in" | "out",
	message: ProjectSocketMessage | ProjectSocketRequest,
): string {
	const arrow = direction === "out" ? "→" : "←";
	const parts = [
		arrow,
		"type" in message ? message.type : "unknown",
		"stream" in message ? message.stream : "unknown",
	];
	if ("event" in message && message.event) parts.push(message.event);
	if ("sessionId" in message && message.sessionId)
		parts.push(message.sessionId);
	if ("threadId" in message && message.threadId) parts.push(message.threadId);
	if ("serviceId" in message && message.serviceId)
		parts.push(message.serviceId);
	return parts.join(" ");
}

function requestMethod(input: RequestInfo | URL, init?: RequestInit): string {
	if (init?.method) return init.method.toUpperCase();
	if (typeof Request !== "undefined" && input instanceof Request) {
		return input.method.toUpperCase();
	}
	return "GET";
}

function requestUrl(input: RequestInfo | URL): string {
	if (typeof Request !== "undefined" && input instanceof Request) {
		return input.url;
	}
	if (input instanceof URL) return input.toString();
	return String(input);
}

function serialize(value: unknown): string {
	try {
		const serialized = JSON.stringify(value, replacer, "\t") ?? "undefined";
		return truncate(serialized);
	} catch (error) {
		return formatError(error);
	}
}

function stringifyStateSnapshot(context: DebugContext): string {
	try {
		const seen = new WeakSet<object>();
		return JSON.stringify(
			{
				view: context.view,
				data: context.data,
			},
			(key, value: unknown) => {
				if (key === "debug") return "[debug omitted]";
				if (typeof value === "function") return "[Function]";
				if (typeof value === "object" && value !== null) {
					if (seen.has(value)) return "[Circular]";
					seen.add(value);
				}
				return value;
			},
		);
	} catch (error) {
		return JSON.stringify({
			error: formatError(error),
		});
	}
}

function captureCommandCallStack(): string | null {
	const stack = new Error().stack;
	if (!stack) return null;
	const frames = stack
		.split("\n")
		.filter(
			(frame) =>
				!frame.includes("captureCommandCallStack") &&
				!frame.includes("logDebugCommandStart"),
		);
	const callStack = frames.join("\n").trim();
	if (!callStack) return null;
	return callStack.length > MAX_COMMAND_CALL_STACK_LENGTH
		? `${callStack.slice(0, MAX_COMMAND_CALL_STACK_LENGTH)}…`
		: callStack;
}

function diffJsonValues(
	before: unknown,
	after: unknown,
	path = "$",
	changes: DebugStateDiffEntry[] = [],
): DebugStateDiffEntry[] {
	if (changes.length >= MAX_DEBUG_STATE_DIFFS_PER_ENTRY) return changes;
	if (Object.is(before, after)) return changes;

	if (Array.isArray(before) && Array.isArray(after)) {
		const length = Math.max(before.length, after.length);
		for (let index = 0; index < length; index += 1) {
			if (index >= before.length) {
				pushStateDiff(
					changes,
					`${path}[${index}]`,
					"added",
					undefined,
					after[index],
				);
			} else if (index >= after.length) {
				pushStateDiff(
					changes,
					`${path}[${index}]`,
					"removed",
					before[index],
					undefined,
				);
			} else {
				diffJsonValues(
					before[index],
					after[index],
					`${path}[${index}]`,
					changes,
				);
			}
		}
		return changes;
	}

	if (isPlainObject(before) && isPlainObject(after)) {
		const keys = new Set([...Object.keys(before), ...Object.keys(after)]);
		for (const key of keys) {
			const childPath = `${path}.${key}`;
			if (!(key in before)) {
				pushStateDiff(changes, childPath, "added", undefined, after[key]);
			} else if (!(key in after)) {
				pushStateDiff(changes, childPath, "removed", before[key], undefined);
			} else {
				diffJsonValues(before[key], after[key], childPath, changes);
			}
		}
		return changes;
	}

	pushStateDiff(changes, path, "changed", before, after);
	return changes;
}

function pushStateDiff(
	changes: DebugStateDiffEntry[],
	path: string,
	type: DebugStateDiffEntry["type"],
	before: unknown,
	after: unknown,
): void {
	if (changes.length >= MAX_DEBUG_STATE_DIFFS_PER_ENTRY) return;
	changes.push({
		path,
		type,
		before: before === undefined ? "undefined" : serialize(before),
		after: after === undefined ? "undefined" : serialize(after),
	});
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function formatConsoleArg(value: unknown): string {
	if (value instanceof Error) return formatErrorWithStack(value);
	if (typeof value === "string") return truncate(value);
	return serialize(value);
}

function formatErrorWithStack(error: unknown): string {
	if (error instanceof Error) {
		return [error.message, error.stack].filter(Boolean).join("\n\n");
	}
	return formatConsoleArg(error);
}

function truncate(value: string): string {
	return value.length > MAX_SERIALIZED_LENGTH
		? `${value.slice(0, MAX_SERIALIZED_LENGTH)}…`
		: value;
}

function replacer(_key: string, value: unknown): unknown {
	if (typeof value === "function") return "[Function]";
	if (value instanceof Event) return `[${value.constructor.name}]`;
	return value;
}

function formatError(error: unknown): string {
	return error instanceof Error ? error.message : String(error);
}

function now(): string {
	return new Date().toISOString();
}

function nextId(): number {
	return nextDebugId++;
}
