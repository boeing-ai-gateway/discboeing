import assert from "node:assert/strict";
import { test } from "vitest";

import {
	getPendingWorkspaceRequiresSourceInput,
	getPendingWorkspaceSourceIsValid,
	getPendingWorkspaceValidationMessage,
} from "../pending-workspace-helpers";

type PendingWorkspace = Parameters<typeof getPendingWorkspaceSourceIsValid>[0];

function pendingWorkspace(
	overrides: Partial<NonNullable<PendingWorkspace>> = {},
): NonNullable<PendingWorkspace> {
	return {
		option: "new-workspace",
		branch: "",
		sourceInput: "",
		validation: null,
		validating: false,
		setupMessage: null,
		sandboxProviderId: "",
		...overrides,
	};
}

test("pending workspace source input is required for explicit local or git sources", () => {
	assert.equal(
		getPendingWorkspaceRequiresSourceInput(
			pendingWorkspace({ option: "local-directory" }),
		),
		true,
	);
	assert.equal(
		getPendingWorkspaceRequiresSourceInput(
			pendingWorkspace({ option: "git-repo" }),
		),
		true,
	);
	assert.equal(
		getPendingWorkspaceRequiresSourceInput(pendingWorkspace()),
		false,
	);
});

test("pending workspace source validity matches the former facade behavior", () => {
	assert.equal(getPendingWorkspaceSourceIsValid(pendingWorkspace()), true);
	assert.equal(
		getPendingWorkspaceSourceIsValid(
			pendingWorkspace({ option: "git-repo", sourceInput: "" }),
		),
		false,
	);
	assert.equal(
		getPendingWorkspaceSourceIsValid(
			pendingWorkspace({
				option: "git-repo",
				sourceInput: "https://example.invalid/repo.git",
				validating: true,
			}),
		),
		false,
	);
	assert.equal(
		getPendingWorkspaceSourceIsValid(
			pendingWorkspace({
				option: "git-repo",
				sourceInput: "https://example.invalid/repo.git",
				validation: {
					path: "https://example.invalid/repo.git",
					sourceType: "git",
					valid: true,
					classification: "cloneable",
					suggestions: [],
				},
			}),
		),
		true,
	);
});

test("pending workspace validation messages describe progress and outcomes", () => {
	assert.equal(getPendingWorkspaceValidationMessage(pendingWorkspace()), null);
	assert.equal(
		getPendingWorkspaceValidationMessage(
			pendingWorkspace({
				option: "local-directory",
				sourceInput: "/tmp/workspace",
				validating: true,
			}),
		),
		"Validating workspace...",
	);
	assert.equal(
		getPendingWorkspaceValidationMessage(
			pendingWorkspace({
				option: "local-directory",
				sourceInput: "/tmp/workspace",
				validation: {
					path: "/tmp/workspace",
					sourceType: "local",
					valid: false,
					classification: "invalid",
					error: "Not a workspace",
					suggestions: [],
				},
			}),
		),
		"Not a workspace",
	);
	assert.equal(
		getPendingWorkspaceValidationMessage(
			pendingWorkspace({
				option: "git-repo",
				sourceInput: "https://example.invalid/repo.git",
				validation: {
					path: "https://example.invalid/repo.git",
					sourceType: "git",
					valid: true,
					classification: "cloneable",
					suggestions: [],
				},
			}),
		),
		"Repository is cloneable.",
	);
});
