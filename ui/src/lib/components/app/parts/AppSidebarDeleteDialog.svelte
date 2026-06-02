<script lang="ts">
	import {
		AlertDialog,
		AlertDialogAction,
		AlertDialogCancel,
		AlertDialogContent,
		AlertDialogDescription,
		AlertDialogFooter,
		AlertDialogHeader,
		AlertDialogTitle,
	} from "$lib/components/ui/alert-dialog";

	type Props = {
		open: boolean;
		title: string;
		description: string;
		deleting?: boolean;
		onCancel: () => void;
		onDelete: () => void;
	};

	let {
		open = $bindable(false),
		title,
		description,
		deleting = false,
		onCancel,
		onDelete,
	}: Props = $props();

	function handleDelete(event: Event) {
		event.preventDefault();
		onDelete();
	}
</script>

<AlertDialog bind:open>
	<AlertDialogContent>
		<AlertDialogHeader>
			<AlertDialogTitle>{title}</AlertDialogTitle>
			<AlertDialogDescription>{description}</AlertDialogDescription>
		</AlertDialogHeader>
		<AlertDialogFooter>
			<AlertDialogCancel onclick={onCancel} disabled={deleting}>
				Cancel
			</AlertDialogCancel>
			<AlertDialogAction onclick={handleDelete} disabled={deleting}>
				Delete
			</AlertDialogAction>
		</AlertDialogFooter>
	</AlertDialogContent>
</AlertDialog>
