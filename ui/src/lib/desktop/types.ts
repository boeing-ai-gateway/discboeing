export type DesktopRuntimeKind = "browser" | "electron";
export type WindowControlsSide = "left" | "right";

export type DesktopServerConfig = {
	port: number;
	secret: string;
};

export type DownloadFileOptions = {
	filename: string;
	content: string | Uint8Array | ArrayBuffer;
	mimeType?: string;
};

export type DesktopWindow = {
	minimize: () => Promise<void>;
	maximize: () => Promise<void>;
	unmaximize: () => Promise<void>;
	isMaximized: () => Promise<boolean>;
	close: () => Promise<void>;
	isFullscreen: () => Promise<boolean>;
	onResized: (listener: () => void) => Promise<() => void>;
};

export type DesktopWindowCallback<T> = (
	window: DesktopWindow,
) => T | Promise<T>;

export type DesktopDownloadEvent =
	| {
			event: "Started";
			data: {
				contentLength?: number;
			};
	  }
	| {
			event: "Progress";
			data: {
				chunkLength: number;
			};
	  }
	| {
			event: "Finished";
	  };

export type DesktopFindInPageOptions = {
	forward?: boolean;
	findNext?: boolean;
};

export type DesktopFindInPageResult = {
	requestId: number;
	activeMatchOrdinal: number;
	matches: number;
	selectionArea?: {
		x: number;
		y: number;
		width: number;
		height: number;
	};
	finalUpdate: boolean;
};

export type DesktopUpdateMetadata = {
	rid: number;
	currentVersion: string;
	version: string;
	date?: string;
	body?: string;
	rawJson: Record<string, unknown>;
};

export type DesktopRendererBridge = {
	kind: "electron";
	initServerConfig?: () => Promise<DesktopServerConfig | null>;
	downloadFile?: (filename: string, bytes: Uint8Array) => Promise<string>;
	readClipboardText?: () => Promise<string>;
	writeClipboardText?: (text: string) => Promise<void>;
	openExternalUrl?: (url: string) => Promise<void>;
	pickDirectory?: () => Promise<string | null>;
	windowMinimize?: () => Promise<void>;
	windowMaximize?: () => Promise<void>;
	windowUnmaximize?: () => Promise<void>;
	windowIsMaximized?: () => Promise<boolean>;
	windowClose?: () => Promise<void>;
	windowIsFullscreen?: () => Promise<boolean>;
	findInPage?: (
		text: string,
		options?: DesktopFindInPageOptions,
	) => Promise<number>;
	stopFindInPage?: (
		action: "clearSelection" | "keepSelection" | "activateSelection",
	) => Promise<void>;
	onFindInPageResult?: (
		listener: (result: DesktopFindInPageResult) => void,
	) => (() => void) | Promise<() => void>;
	onWindowResized?: (
		listener: () => void,
	) => (() => void) | Promise<() => void>;
	checkForAppUpdate?: (
		endpoint?: string | null,
	) => Promise<DesktopUpdateMetadata | null>;
	downloadAppUpdate?: (
		rid: number,
		onEvent: (event: DesktopDownloadEvent) => void,
	) => Promise<number>;
	installAppUpdate?: (updateRid: number, bytesRid: number) => Promise<void>;
	closeAppUpdate?: (
		updateRid?: number | null,
		bytesRid?: number | null,
	) => Promise<void>;
	relaunchApp?: () => Promise<void>;
};
