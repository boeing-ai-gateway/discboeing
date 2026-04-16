import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const SUBMIT_BUTTON_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/parts/ConversationComposerSubmitButton.svelte",
);

const CONVERSATION_COMPOSER_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/ConversationComposer.svelte",
);

const CONVERSATION_CREDENTIALS_CONTROL_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/ConversationCredentialsControl.svelte",
);

function readSubmitButtonSource() {
	return readFileSync(SUBMIT_BUTTON_COMPONENT, "utf-8");
}

function readConversationComposerSource() {
	return readFileSync(CONVERSATION_COMPOSER_COMPONENT, "utf-8");
}

function readConversationCredentialsControlSource() {
	return readFileSync(CONVERSATION_CREDENTIALS_CONTROL_COMPONENT, "utf-8");
}

test("composer submit button only shows the plus icon for pending empty sessions", () => {
	const source = readSubmitButtonSource();

	assert.match(source, /isPending: boolean;/);
	assert.match(
		source,
		/let\s+\{[\s\S]*status,[\s\S]*inputEmpty,[\s\S]*isPending,[\s\S]*disabled = false,[\s\S]*onPress,[\s\S]*\}: Props = \$props\(\);/,
	);
	assert.match(source, /hovered && isPending && inputEmpty && !isGenerating/);
	assert.doesNotMatch(source, /hovered && inputEmpty && !isGenerating/);
});

test("conversation composer passes pending session state to the submit button", () => {
	const source = readConversationComposerSource();

	assert.match(
		source,
		/<ConversationComposerSubmitButton[\s\S]*isPending=\{session\.isPending\}/,
	);
});

test("conversation composer disables input when no models are available", () => {
	const source = readConversationComposerSource();

	assert.match(
		source,
		/const hasAvailableModels = \$derived\.by\(\(\) => models\.list\.length > 0\);/,
	);
	assert.match(source, /if \(!hasAvailableModels\) \{/);
	assert.match(source, /Please add a valid LLM provider credential/);
	assert.match(source, /onclick=\{ui\.openCredentialsDialog\}/);
	assert.match(
		source,
		/<ConversationComposerTextarea[\s\S]*disabled=\{composerDisabled\}/,
	);
	assert.match(
		source,
		/<ConversationComposerAttachmentButton[\s\S]*disabled=\{composerDisabled\}/,
	);
});

test("conversation composer keeps session credentials interactive during pending questions", () => {
	const source = readConversationComposerSource();

	assert.match(
		source,
		/Answer the agent's pending question before sending a new message\./,
	);
	assert.match(source, /<ConversationCredentialsControl \/>/);
	assert.doesNotMatch(
		source,
		/class=\{composerDisabled \? "pointer-events-none opacity-60" : undefined\}/,
	);
});

test("session credentials control only shows a numeric badge when multiple credentials are visible", () => {
	const source = readConversationCredentialsControlSource();

	assert.match(source, /visibleCount > 1 \? "text-foreground" : "invisible"/);
	assert.match(source, /\{visibleCount\}/);
	assert.doesNotMatch(source, /visibleCount\/\{availableCredentials\.length\}/);
});

test("session credential use deletion removes the binding when the last use is deleted", () => {
	const source = readConversationCredentialsControlSource();

	assert.match(
		source,
		/const remainingUses = \(assignment\.uses \?\? \[\]\)\.filter\(/,
	);
	assert.match(
		source,
		/if \(remainingUses\.length === 0\) \{\s*return \[\];\s*\}/,
	);
});

test("opening global credential management closes the session credentials menu", () => {
	const source = readConversationCredentialsControlSource();

	assert.match(source, /let dropdownOpen = \$state\(false\);/);
	assert.match(source, /<DropdownMenu bind:open=\{dropdownOpen\}>/);
	assert.match(
		source,
		/closeGlobalVisibilityDialog\(\);\s*dropdownOpen = false;\s*if \(!credentialId\)/,
	);
});
