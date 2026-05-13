import type { Session } from "../api-types";

export type RecentThreadEntry = {
	sessionId: string;
	threadId: string;
	name: string;
	lastAccessedAt: string;
};

function compareIsoDatesDesc(left: string, right: string) {
	const leftTime = new Date(left).getTime();
	const rightTime = new Date(right).getTime();
	if (Number.isNaN(leftTime) || Number.isNaN(rightTime)) {
		return 0;
	}
	return rightTime - leftTime;
}

export function recentThreadKey(sessionId: string, threadId: string): string {
	return `${sessionId}:${threadId}`;
}

type ThreadSwitcherSession = Pick<
	Session,
	"id" | "name" | "displayName" | "createdAt" | "status"
>;

export function getAvailableSwitcherThreads(args: {
	sessions: ThreadSwitcherSession[];
	recentThreads: RecentThreadEntry[];
}): RecentThreadEntry[] {
	const trackedSessionIds = Object.fromEntries(
		args.recentThreads.map((thread) => [thread.sessionId, true] as const),
	);

	return [
		...args.recentThreads,
		...args.sessions.flatMap((session) =>
			trackedSessionIds[session.id]
				? []
				: [
						{
							sessionId: session.id,
							threadId: session.id,
							name: session.displayName || session.name,
							lastAccessedAt: session.createdAt,
						},
					],
		),
	].sort((left, right) =>
		compareIsoDatesDesc(left.lastAccessedAt, right.lastAccessedAt),
	);
}

export function getThreadSwitcherThreads(args: {
	threads: RecentThreadEntry[];
	selectedThreadKey: string | null;
}): RecentThreadEntry[] {
	const sortedThreads = [...args.threads].sort((left, right) =>
		compareIsoDatesDesc(left.lastAccessedAt, right.lastAccessedAt),
	);

	if (!args.selectedThreadKey) {
		return sortedThreads;
	}

	const selectedIndex = sortedThreads.findIndex(
		(thread) =>
			recentThreadKey(thread.sessionId, thread.threadId) ===
			args.selectedThreadKey,
	);
	if (selectedIndex <= 0) {
		return sortedThreads;
	}

	const [selectedThread] = sortedThreads.splice(selectedIndex, 1);
	return selectedThread ? [selectedThread, ...sortedThreads] : sortedThreads;
}
