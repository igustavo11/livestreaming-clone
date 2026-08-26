import { z } from "zod";

// Mirrors the JSON contracts in internal/auth, internal/channel and internal/public.
// Parsing every response through these catches backend/frontend contract drift
// immediately instead of rendering `undefined` silently.

export const UserSchema = z.object({
	id: z.string(),
	email: z.string(),
	username: z.string(),
});
export type User = z.infer<typeof UserSchema>;

const coreChannelShape = {
	id: z.string(),
	username: z.string(),
	title: z.string(),
	category: z.string(),
	avatar_url: z.string(),
	is_live: z.boolean(),
};

export const AuthChannelSchema = z.object(coreChannelShape);
export type AuthChannel = z.infer<typeof AuthChannelSchema>;

export const PublicChannelSchema = z.object({
	...coreChannelShape,
	thumbnail_url: z.string(),
	viewer_count: z.number(),
	follower_count: z.number(),
	is_following: z.boolean(),
});
export type PublicChannel = z.infer<typeof PublicChannelSchema>;

export const DashboardChannelSchema = z.object({
	...coreChannelShape,
	thumbnail_url: z.string(),
	stream_key_preview: z.string(),
});
export type DashboardChannel = z.infer<typeof DashboardChannelSchema>;

export const AuthResponseSchema = z.object({
	user: UserSchema,
	channel: AuthChannelSchema,
});
export type AuthResponse = z.infer<typeof AuthResponseSchema>;

export const ChannelListResponseSchema = z.object({
	channels: z.array(PublicChannelSchema),
	has_more: z.boolean(),
});

export const PublicChannelResponseSchema = z.object({
	channel: PublicChannelSchema,
});

export const DashboardChannelResponseSchema = z.object({
	channel: DashboardChannelSchema,
});

export const RotateStreamKeyResponseSchema = z.object({
	stream_key: z.string(),
	channel: DashboardChannelSchema,
});

export const FollowResponseSchema = z.object({
	follower_count: z.number(),
	is_following: z.boolean(),
});
