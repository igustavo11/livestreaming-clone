import { type ChangeEvent, useRef } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { useUploadAvatar } from "@/modules/dashboard/hooks/use-upload-avatar";
import { ChannelAvatar } from "@/shared/components/channel/ChannelAvatar";
import { ApiError } from "@/shared/lib/api-client";

type AvatarUploaderProps = {
	username: string;
	avatarUrl: string;
};

export function AvatarUploader({ username, avatarUrl }: AvatarUploaderProps) {
	const uploadAvatar = useUploadAvatar();
	const inputRef = useRef<HTMLInputElement>(null);

	function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
		const file = event.target.files?.[0];
		event.target.value = "";
		if (!file) return;
		uploadAvatar.mutate(file, {
			onSuccess: () => toast.success("Foto de perfil atualizada"),
			onError: (err) =>
				toast.error(
					err instanceof ApiError ? err.message : "Falha ao enviar imagem",
				),
		});
	}

	return (
		<>
			<Button
				type="button"
				variant="ghost"
				onClick={() => inputRef.current?.click()}
				disabled={uploadAvatar.isPending}
				aria-label="Trocar foto de perfil"
				className="size-14 shrink-0 rounded-full p-0"
			>
				<ChannelAvatar
					username={username}
					avatarUrl={avatarUrl}
					className="size-14"
				/>
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
