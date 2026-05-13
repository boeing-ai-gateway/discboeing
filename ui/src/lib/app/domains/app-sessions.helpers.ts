import type { Session } from "$lib/api-types";

function compareIsoDatesDesc(left: string, right: string) {
	const leftTime = new Date(left).getTime();
	const rightTime = new Date(right).getTime();
	if (Number.isNaN(leftTime) || Number.isNaN(rightTime)) {
		return 0;
	}
	return rightTime - leftTime;
}

export function sortSessionsByCreatedAt(sessions: Session[]): Session[] {
	return [...sessions].sort((a, b) =>
		compareIsoDatesDesc(a.createdAt, b.createdAt),
	);
}

export function getNextSelectedSessionId(
	sessions: Pick<Session, "id">[],
	removedSessionId: string,
	currentSelectedSessionId: string | null,
): string | null {
	if (currentSelectedSessionId !== removedSessionId) {
		return currentSelectedSessionId;
	}

	return (
		sessions.find((session) => session.id !== removedSessionId)?.id ?? null
	);
}

export function getReconciledSelectedSessionId(
	sessions: Pick<Session, "id">[],
	currentSelectedSessionId: string | null,
	explicitSelectedSessionId?: string | null,
): string | null {
	if (
		explicitSelectedSessionId &&
		sessions.some((session) => session.id === explicitSelectedSessionId)
	) {
		return explicitSelectedSessionId;
	}

	if (
		currentSelectedSessionId &&
		sessions.some((session) => session.id === currentSelectedSessionId)
	) {
		return currentSelectedSessionId;
	}

	return null;
}

export function upsertSession(
	sessions: Session[],
	nextSession: Session,
): Session[] {
	const existingIndex = sessions.findIndex(
		(session) => session.id === nextSession.id,
	);
	if (existingIndex === -1) {
		return [...sessions, nextSession];
	}

	return sessions.map((session, index) =>
		index === existingIndex ? nextSession : session,
	);
}
