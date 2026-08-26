import { type ChangeEvent, useRef } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { useUploadThumbnail } from "@/modules/dashboard/hooks/use-upload-thumbnail";
import { ApiError } from "@/shared/lib/api-client";

export function ThumbnailUploader({ thumbnailUrl }: { thumbnailUrl: string }) {
	const uploadThumbnail = useUploadThumbnail();
	const inputRef = useRef<HTMLInputElement>(null);

	function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
		const file = event.target.files?.[0];
		event.target.value = "";
		if (!file) return;
		uploadThumbnail.mutate(file, {
			onSuccess: () => toast.success("Thumbnail atualizada"),
			onError: (err) =>
				toast.error(
					err instanceof ApiError ? err.message : "Falha ao enviar thumbnail",
				),
		});
	}

	return (
		<>
			<Button
				type="button"
				variant="ghost"
				onClick={() => inputRef.current?.click()}
				disabled={uploadThumbnail.isPending}
				className="h-auto w-full max-w-70 gap-0 overflow-hidden rounded-xl bg-secondary p-0 hover:bg-secondary sm:w-70"
			>
				<span className="relative block aspect-video w-full">
					{thumbnailUrl ? (
						<img
							src={thumbnailUrl}
							alt="Thumbnail do canal"
							className="size-full object-cover"
						/>
					) : (
						<span className="flex size-full items-center justify-center text-xs text-muted-foreground">
							{uploadThumbnail.isPending ? "Enviando..." : "Trocar thumbnail"}
						</span>
					)}
				</span>
			</Button>
			<input
				ref={inputRef}
				type="file"
				accept="image/jpeg,image/png,image/webp"
				className="hidden"
				onChange={handleFileChange}
			/>
		</>
	);
}
