import * as Sentry from "@sentry/sveltekit";
import { env as publicEnv } from "$env/dynamic/public";

export const init = () => {
	const envSummary = Object.fromEntries(
		Object.entries(publicEnv).sort(([left], [right]) =>
			left.localeCompare(right),
		),
	);
	const dsn = publicEnv.PUBLIC_SENTRY_DSN;
	const release = publicEnv.PUBLIC_SENTRY_RELEASE;
	const dist = publicEnv.PUBLIC_SENTRY_DIST;
	const gitCommit = publicEnv.PUBLIC_SENTRY_GIT_COMMIT;
	const gitTag = publicEnv.PUBLIC_SENTRY_GIT_TAG;
	const appVersion = publicEnv.PUBLIC_SENTRY_APP_VERSION;
	const sentryEnabled = Boolean(dsn);

	const tags = {
		...(appVersion ? { app_version: appVersion } : {}),
		...(gitCommit ? { git_commit: gitCommit } : {}),
		...(gitTag ? { git_tag: gitTag } : {}),
	};

	Sentry.init({
		dsn,
		enabled: sentryEnabled,
		environment: import.meta.env.MODE,
		...(release ? { release } : {}),
		...(dist ? { dist } : {}),
		initialScope: {
			tags,
		},
	});

	console.info("[env] import.meta.env", envSummary);
	console.info("[sentry] initialized", {
		enabled: sentryEnabled,
		environment: import.meta.env.MODE,
		...(release ? { release } : {}),
		...(dist ? { dist } : {}),
	});
};

export const handleError = Sentry.handleErrorWithSentry();
