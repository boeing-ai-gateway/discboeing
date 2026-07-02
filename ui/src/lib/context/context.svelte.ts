import {
	getContext as getSvelteContext,
	setContext as setSvelteContext,
} from "svelte";
import { MediaQuery } from "svelte/reactivity";

import { createCommands } from "$lib/context/commands";
import { syncRecentThreads } from "$lib/context/domains/recent-threads";
import type { Bootstrap, Context } from "$lib/context/context.types";
import {
	createInitialDataState,
	createInitialViewState,
} from "$lib/context/initial-state";
import { detectIsMacPlatform } from "$lib/shortcuts/global-shortcuts";

const CONTEXT_KEY = Symbol.for("discboeing-ui-context");
const MOBILE_BREAKPOINT = 1024;

let currentContext: Context | null = null;

export function createContext(bootstrap: Bootstrap = {}): Context {
	const mobileQuery = new MediaQuery(`max-width: ${MOBILE_BREAKPOINT - 1}px`);
	const context = $state<Context>({
		data: createInitialDataState(bootstrap),
		view: createInitialViewState(bootstrap),
		commands: undefined as unknown as Context["commands"],
	});

	context.view.app.environment.isMobile = mobileQuery.current;
	context.view.app.environment.isMacPlatform = detectIsMacPlatform();
	context.commands = createCommands(context);
	currentContext = context;

	$effect.root(() => {
		$effect(() => {
			context.view.app.environment.isMobile = mobileQuery.current;
		});

		$effect(() => {
			syncRecentThreads(context);
		});
	});

	return context;
}

export function setContext(context: Context): Context {
	currentContext = context;
	setSvelteContext(CONTEXT_KEY, context);
	return context;
}

export function useContext(): Context {
	const context = getSvelteContext<Context | undefined>(CONTEXT_KEY);
	if (!context) {
		throw new Error("useContext must be used within Context provider");
	}
	return context;
}

export function getContext(): Context {
	if (!currentContext) {
		throw new Error("context has not been created");
	}
	return currentContext;
}
