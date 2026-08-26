import { z } from "zod";

// Mirrors internal/auth/username.go and internal/auth/password.go exactly,
// so the browser rejects an invalid signup before it ever reaches the API.
const usernameSchema = z
	.string()
	.regex(
		/^[a-z0-9_]{3,25}$/,
		"3-25 caracteres: letras minúsculas, números e _",
	);

const passwordSchema = z
	.string()
	.min(8, "Senha deve ter no mínimo 8 caracteres");

export const loginSchema = z.object({
	email: z.string().email("Email inválido"),
	password: z.string().min(1, "Informe sua senha"),
});
export type LoginInput = z.infer<typeof loginSchema>;

export const signupSchema = z.object({
	email: z.string().email("Email inválido"),
	username: usernameSchema,
	password: passwordSchema,
});
export type SignupInput = z.infer<typeof signupSchema>;

export const forgotPasswordSchema = z.object({
	email: z.string().email("Email inválido"),
});
export type ForgotPasswordInput = z.infer<typeof forgotPasswordSchema>;

export const resetPasswordSchema = z.object({
	token: z.string().min(1),
	password: passwordSchema,
});
export type ResetPasswordInput = z.infer<typeof resetPasswordSchema>;

export const onboardingSchema = z.object({
	username: usernameSchema,
});
export type OnboardingInput = z.infer<typeof onboardingSchema>;
