import { mount } from "svelte";
import App from "./app/App.svelte";
import "./styles.css";

const target = document.getElementById("app");
if (!target) {
	throw new Error("#app is missing");
}

mount(App, { target });
