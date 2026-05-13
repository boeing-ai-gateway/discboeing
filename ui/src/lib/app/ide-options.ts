export const SUPPORTED_PREFERRED_IDES = [
	"cursor",
	"vscode",
	"zed",
	"jetbrains-intellij-idea",
	"jetbrains-webstorm",
	"jetbrains-goland",
	"jetbrains-pycharm",
	"jetbrains-phpstorm",
	"jetbrains-clion",
	"jetbrains-rubymine",
	"jetbrains-rider",
] as const;

export type PreferredIde = (typeof SUPPORTED_PREFERRED_IDES)[number];
export type StandardPreferredIde = Extract<
	PreferredIde,
	"cursor" | "vscode" | "zed"
>;
export type JetBrainsPreferredIde = Exclude<PreferredIde, StandardPreferredIde>;
export type JetBrainsProductCode =
	| "IU"
	| "WS"
	| "GO"
	| "PY"
	| "PS"
	| "CL"
	| "RM"
	| "RD";

type BaseIdeOption = {
	id: PreferredIde;
	label: string;
};

export type StandardIdeOption = BaseIdeOption & {
	family: "standard";
};

export type JetBrainsIdeOption = BaseIdeOption & {
	family: "jetbrains";
	productCode: JetBrainsProductCode;
};

export type IdeOption = StandardIdeOption | JetBrainsIdeOption;

export function isPreferredIde(
	value: string | null | undefined,
): value is PreferredIde {
	return (SUPPORTED_PREFERRED_IDES as readonly string[]).includes(value ?? "");
}
