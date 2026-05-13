<script lang="ts">
	import { Input } from "$lib/components/ui/input";
	import { Button } from "$lib/components/ui/button";

	type Props = {
		currentRunAfter?: string;
		disabled?: boolean;
		onSelect: (runAfter: Date | null) => void | Promise<void>;
	};

	let { currentRunAfter, disabled = false, onSelect }: Props = $props();

	function parseRunAfter(value?: string): Date | null {
		if (!value) {
			return null;
		}
		const parsed = new Date(value);
		return Number.isNaN(parsed.getTime()) ? null : parsed;
	}

	function padDatePart(value: number): string {
		return value.toString().padStart(2, "0");
	}

	function toDateTimeLocalValue(date: Date): string {
		return `${date.getFullYear()}-${padDatePart(date.getMonth() + 1)}-${padDatePart(
			date.getDate(),
		)}T${padDatePart(date.getHours())}:${padDatePart(date.getMinutes())}`;
	}

	function buildPauseDate(): Date {
		const now = new Date();
		return new Date(
			now.getFullYear() + 100,
			now.getMonth(),
			now.getDate(),
			now.getHours(),
			now.getMinutes(),
			now.getSeconds(),
			now.getMilliseconds(),
		);
	}

	function getDefaultCustomLaterValue(): string {
		const current = parseRunAfter(currentRunAfter);
		if (current) {
			return toDateTimeLocalValue(current);
		}
		return toDateTimeLocalValue(new Date(Date.now() + 60 * 60 * 1000));
	}

	let customLaterValue = $derived(getDefaultCustomLaterValue());

	async function saveCustomLater() {
		const value = customLaterValue.trim();
		if (!value) {
			await onSelect(null);
			return;
		}
		const parsed = new Date(value);
		if (Number.isNaN(parsed.getTime())) {
			return;
		}
		await onSelect(parsed);
	}
</script>

<div class="space-y-3">
	<div>
		<div class="text-sm font-medium">Run later</div>
		<div class="text-xs text-muted-foreground">
			Pick when this prompt becomes eligible.
		</div>
	</div>
	<div class="grid grid-cols-2 gap-2">
		<Button
			variant="outline"
			size="xs"
			onclick={() => void onSelect(new Date(Date.now() + 15 * 60 * 1000))}
			{disabled}
		>
			15 min
		</Button>
		<Button
			variant="outline"
			size="xs"
			onclick={() => void onSelect(new Date(Date.now() + 60 * 60 * 1000))}
			{disabled}
		>
			1 hour
		</Button>
		<Button
			variant="outline"
			size="xs"
			onclick={() => void onSelect(new Date(Date.now() + 24 * 60 * 60 * 1000))}
			{disabled}
		>
			1 day
		</Button>
		<Button
			variant="outline"
			size="xs"
			onclick={() => void onSelect(buildPauseDate())}
			{disabled}
		>
			Pause
		</Button>
	</div>
	<div class="space-y-2">
		<div class="text-xs font-medium text-muted-foreground">Custom time</div>
		<Input
			type="datetime-local"
			value={customLaterValue}
			oninput={(event) => {
				customLaterValue = event.currentTarget.value;
			}}
		/>
		<div class="flex justify-between gap-2">
			<Button
				variant="outline"
				size="xs"
				onclick={() => void onSelect(null)}
				{disabled}
			>
				Run now
			</Button>
			<Button size="xs" onclick={() => void saveCustomLater()} {disabled}>
				Save
			</Button>
		</div>
	</div>
</div>
