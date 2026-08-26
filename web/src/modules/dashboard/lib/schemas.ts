import { z } from "zod";
import { CATEGORY_SLUGS } from "@/shared/lib/categories";

export const updateChannelSchema = z.object({
	title: z.string().max(140, "Título deve ter no máximo 140 caracteres"),
	category: z.enum(CATEGORY_SLUGS, {
		message: "Selecione uma categoria válida",
	}),
});
export type UpdateChannelInput = z.infer<typeof updateChannelSchema>;
