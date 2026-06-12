import {
	siCursor,
	siJetbrains,
	siZedindustries,
	type SimpleIcon,
} from "simple-icons";
import type { IdeIcon, IdeOption } from "$lib/shell/ide-options";

export const windowControls = ["_", "□", "×"];

const vscodeIcon: IdeIcon = {
	title: "Visual Studio Code",
	path: "M23.3 4.4 18.7 2a1.5 1.5 0 0 0-1.7.3L8.2 10 4.4 7.1a1 1 0 0 0-1.3.1L1.4 8.7a1 1 0 0 0 0 1.5L4.7 13l-3.3 2.8a1 1 0 0 0 0 1.5l1.7 1.5a1 1 0 0 0 1.3.1l3.8-2.9 8.8 7.7a1.5 1.5 0 0 0 1.7.3l4.6-2.4a1.5 1.5 0 0 0 .8-1.3V5.7a1.5 1.5 0 0 0-.8-1.3ZM18 17.7 11.1 13 18 8.3v9.4Z",
};

function simpleIcon(icon: SimpleIcon): IdeIcon {
	return { title: icon.title, path: icon.path };
}

const cursorIcon = simpleIcon(siCursor);
const jetbrainsIcon = simpleIcon(siJetbrains);
const zedIcon = simpleIcon(siZedindustries);

export const ideOptions: IdeOption[] = [
	{ id: "cursor", label: "Cursor", family: "standard", icon: cursorIcon },
	{ id: "vscode", label: "VS Code", family: "standard", icon: vscodeIcon },
	{ id: "zed", label: "Zed", family: "standard", icon: zedIcon },
	{
		id: "jetbrains-intellij-idea",
		label: "IntelliJ IDEA Ultimate",
		family: "jetbrains",
		productCode: "IU",
		icon: jetbrainsIcon,
	},
	{
		id: "jetbrains-webstorm",
		label: "WebStorm",
		family: "jetbrains",
		productCode: "WS",
		icon: jetbrainsIcon,
	},
	{
		id: "jetbrains-goland",
		label: "GoLand",
		family: "jetbrains",
		productCode: "GO",
		icon: jetbrainsIcon,
	},
	{
		id: "jetbrains-pycharm",
		label: "PyCharm Professional",
		family: "jetbrains",
		productCode: "PY",
		icon: jetbrainsIcon,
	},
	{
		id: "jetbrains-phpstorm",
		label: "PhpStorm",
		family: "jetbrains",
		productCode: "PS",
		icon: jetbrainsIcon,
	},
	{
		id: "jetbrains-clion",
		label: "CLion",
		family: "jetbrains",
		productCode: "CL",
		icon: jetbrainsIcon,
	},
	{
		id: "jetbrains-rubymine",
		label: "RubyMine",
		family: "jetbrains",
		productCode: "RM",
		icon: jetbrainsIcon,
	},
	{
		id: "jetbrains-rider",
		label: "Rider",
		family: "jetbrains",
		productCode: "RD",
		icon: jetbrainsIcon,
	},
];
