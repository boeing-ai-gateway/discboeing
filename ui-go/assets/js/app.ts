import { installSessionTransportFallback } from "./bootstrap/session-transport";
import { pierreDiffViewer, registerComponents, theme } from "./components";

installSessionTransportFallback();
registerComponents();
theme.watchSystem(() => pierreDiffViewer.enhance());

document.addEventListener("DOMContentLoaded", () => {
	theme.applyStored();
	pierreDiffViewer.enhance();
});

document.addEventListener("datastar-patched", () => {
	pierreDiffViewer.enhance();
});
