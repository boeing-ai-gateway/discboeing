import { z } from "zod";
import { createValidator, type ToolSchema } from "./index";

export const RequestedCredentialApprovedUseSchema = z.object({
	description: z.string(),
});

export const RequestedCredentialItemSchema = z.object({
	envVar: z.string(),
	name: z.string(),
	justification: z.string(),
	approvedUses: z.array(RequestedCredentialApprovedUseSchema),
});

export const RequestUserCredentialToolInputSchema = z.object({
	credentials: z.array(RequestedCredentialItemSchema).default([]),
});

export type RequestUserCredentialToolInput = z.infer<
	typeof RequestUserCredentialToolInputSchema
>;

export const GrantedCredentialApprovedUseSchema = z.object({
	id: z.string(),
	description: z.string(),
});

export const GrantedCredentialSchema = z.object({
	credentialId: z.string(),
	envVar: z.string(),
	name: z.string(),
	approvedUses: z.array(GrantedCredentialApprovedUseSchema),
});

export const RequestUserCredentialToolOutputSchema = z.union([
	z.string(),
	z.object({
		grantedCredentials: z.array(GrantedCredentialSchema).default([]),
	}),
	z.record(z.string(), z.unknown()),
	z.array(z.unknown()),
]);

export type RequestUserCredentialToolOutput = z.infer<
	typeof RequestUserCredentialToolOutputSchema
>;

export const validateRequestUserCredentialInput = createValidator(
	RequestUserCredentialToolInputSchema,
);

export const validateRequestUserCredentialOutput = createValidator(
	RequestUserCredentialToolOutputSchema,
);

export const RequestUserCredentialToolSchema: ToolSchema<
	RequestUserCredentialToolInput,
	RequestUserCredentialToolOutput
> = {
	toolName: "RequestUserCredential",
	inputSchema: RequestUserCredentialToolInputSchema,
	outputSchema: RequestUserCredentialToolOutputSchema,
	validateInput: validateRequestUserCredentialInput,
	validateOutput: validateRequestUserCredentialOutput,
};
