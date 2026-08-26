import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
	Form,
	FormControl,
	FormField,
	FormItem,
	FormLabel,
	FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { ThumbnailUploader } from "@/modules/dashboard/components/ThumbnailUploader";
import { useUpdateChannel } from "@/modules/dashboard/hooks/use-update-channel";
import {
	type UpdateChannelInput,
	updateChannelSchema,
} from "@/modules/dashboard/lib/schemas";
import { ApiError } from "@/shared/lib/api-client";
import {
	CATEGORY_LABELS,
	CATEGORY_SLUGS,
	type CategorySlug,
} from "@/shared/lib/categories";
import type { DashboardChannel } from "@/shared/schemas/channel";

// A fresh channel is created with category="" (see migrations/000002), which
// isn't a valid slug — fall back so the Select always has a real value.
function toCategorySlug(value: string): CategorySlug {
	return (CATEGORY_SLUGS as readonly string[]).includes(value)
		? (value as CategorySlug)
		: "other";
}

export function LiveInfoForm({ channel }: { channel: DashboardChannel }) {
	const updateChannel = useUpdateChannel();
	const form = useForm<UpdateChannelInput>({
		resolver: zodResolver(updateChannelSchema),
		defaultValues: {
			title: channel.title,
			category: toCategorySlug(channel.category),
		},
	});

	useEffect(() => {
		form.reset({
			title: channel.title,
			category: toCategorySlug(channel.category),
		});
	}, [channel.title, channel.category, form]);

	function onSubmit(values: UpdateChannelInput) {
		updateChannel.mutate(values, {
			onSuccess: () => toast.success("Alterações salvas"),
			onError: (err) =>
				toast.error(
					err instanceof ApiError ? err.message : "Não foi possível salvar",
				),
		});
	}

	return (
		<section className="flex flex-col gap-5.5 rounded-2xl border border-border bg-card p-7">
			<h2 className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
				Informações da live
			</h2>
			<Form {...form}>
				<form
					onSubmit={form.handleSubmit(onSubmit)}
					className="flex flex-col gap-5.5"
				>
					<FormField
						control={form.control}
						name="title"
						render={({ field }) => (
							<FormItem>
								<FormLabel>Título</FormLabel>
								<FormControl>
									<Input placeholder="Título da sua live" {...field} />
								</FormControl>
								<FormMessage />
							</FormItem>
						)}
					/>
					<FormField
						control={form.control}
						name="category"
						render={({ field }) => (
							<FormItem>
								<FormLabel>Categoria</FormLabel>
								<Select value={field.value} onValueChange={field.onChange}>
									<FormControl>
										<SelectTrigger className="w-full">
											<SelectValue placeholder="Selecione uma categoria" />
										</SelectTrigger>
									</FormControl>
									<SelectContent>
										{CATEGORY_SLUGS.map((slug) => (
											<SelectItem key={slug} value={slug}>
												{CATEGORY_LABELS[slug]}
											</SelectItem>
										))}
									</SelectContent>
								</Select>
								<FormMessage />
							</FormItem>
						)}
					/>

					<div className="flex flex-col gap-2">
						<span className="text-sm font-semibold text-foreground/70">
							Thumbnail
						</span>
						<ThumbnailUploader thumbnailUrl={channel.thumbnail_url} />
					</div>

					<div className="flex justify-end">
						<Button type="submit" disabled={updateChannel.isPending}>
							{updateChannel.isPending ? "Salvando..." : "Salvar alterações"}
						</Button>
					</div>
				</form>
			</Form>
		</section>
	);
}
