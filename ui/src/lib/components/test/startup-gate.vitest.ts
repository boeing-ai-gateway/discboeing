import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "vitest";

const STARTUP_GATE_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/StartupGate.svelte",
);

function readStartupGateSource() {
	return readFileSync(STARTUP_GATE_COMPONENT, "utf-8");
}

test("startup gate does not mount app children until startup is ready", () => {
	const source = readStartupGateSource();

	assert.match(source, /\{#if ready && startupPhase !== "auth"\}/);
	assert.match(source, /\{@render children\(\)\}/);
	assert.doesNotMatch(source, /opacity-0[\s\S]*\{@render children\(\)\}/);
	assert.match(source, /import \{ useContext \} from "\$lib\/context";/);
	assert.match(source, /const context = useContext\(\);/);
	assert.match(
		source,
		/await context\.commands\.lifecycle\.startup\(\{ wait: true \}\)/,
	);
	assert.doesNotMatch(source, /refreshRuntimeData\(\)/);
	assert.doesNotMatch(source, /connectRuntimeProjectEvents\(\)/);
	assert.doesNotMatch(source, /\$lib\/ng/);
});
