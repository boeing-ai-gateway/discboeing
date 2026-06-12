import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "vitest";

const SESSION_WORKSPACE_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/SessionWorkspace.svelte",
);
const SESSION_ACTIVATION_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/SessionActivation.svelte",
);

function readSessionWorkspaceSource() {
	return readFileSync(SESSION_WORKSPACE_COMPONENT, "utf-8");
}

function readSessionActivationSource() {
	return readFileSync(SESSION_ACTIVATION_COMPONENT, "utf-8");
}

test("session workspace mounts activation when the session becomes ready", () => {
	const source = readSessionWorkspaceSource();

	assert.match(source, /type Props = \{/);
	assert.match(source, /sessionId: string;/);
	assert.match(source, /visible: boolean;/);
	assert.match(source, /mainClass: string;/);
	assert.doesNotMatch(source, /showSidebarToggle\?: boolean;/);
	assert.doesNotMatch(source, /reserveSidebarSpace\?: boolean;/);
	assert.doesNotMatch(source, /\$lib\/context\/runtime/);
	assert.doesNotMatch(source, /ensureRuntimeSessionState/);
	assert.doesNotMatch(source, /releaseRuntimeSessionState/);
	assert.match(source, /import \{ useContext \} from "\$lib\/context";/);
	assert.match(source, /const context = useContext\(\);/);
	assert.match(source, /context\.data\.sessions\.byId\[mountedSessionId\]/);
	assert.match(source, /context\.data\.project\.status\.state === "ready"/);
	assert.match(source, /context\.view\.selection\.pendingSessionId/);
	assert.match(
		source,
		/context\.view\.selection\.requestedThreadIdBySessionId/,
	);
	assert.match(
		source,
		/import SessionActivation from "\$lib\/components\/app\/SessionActivation\.svelte";/,
	);
	assert.match(
		source,
		/\{#if !isPendingWorkspace && projectReady\}[\s\S]*<SessionActivation sessionId=\{mountedSessionId\} \/>[\s\S]*\{\/if\}/,
	);
	assert.doesNotMatch(source, /activateSession\(mountedSessionId\)/);
	assert.doesNotMatch(source, /deactivateSession\(mountedSessionId\)/);
	assert.match(source, /const threadId = \$derived\.by\(/);
	assert.match(source, /\{#key threadId\}/);
	assert.match(source, /<ThreadWorkspace/);
	assert.match(source, /sessionId=\{mountedSessionId\}/);
	assert.match(source, /\{threadId\}/);
	assert.doesNotMatch(source, /\{reserveSidebarSpace\}/);
	assert.doesNotMatch(source, /\{showSidebarToggle\}/);
	assert.match(
		source,
		/mode=\{!currentSession \? "conversation-only" : undefined\}/,
	);
	assert.doesNotMatch(source, /new ResizeObserver/);
	assert.doesNotMatch(source, /<SessionSidebar/);
});

test("session activation deactivates after activation starts", () => {
	const source = readSessionActivationSource();

	assert.match(source, /import \{ onDestroy, onMount \} from "svelte";/);
	assert.match(source, /import \{ useContext \} from "\$lib\/context";/);
	assert.match(source, /let activationStarted = false;/);
	assert.match(source, /activationStarted = true;/);
	assert.match(
		source,
		/void context\.commands\.sessions\s*\.activateSession\(sessionId\)\s*\.catch\(\(\) => undefined\);/,
	);
	assert.match(source, /if \(!activationStarted\) \{[\s\S]*return;[\s\S]*\}/);
	assert.match(
		source,
		/void context\.commands\.sessions\s*\.deactivateSession\(sessionId\)\s*\.catch\(\(\) => undefined\);/,
	);
	assert.doesNotMatch(source, /destroyed/);
	assert.doesNotMatch(
		source,
		/await context\.commands\.sessions\.activateSession/,
	);
});
