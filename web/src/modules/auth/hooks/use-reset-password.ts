import { useMutation } from "@tanstack/react-query";
import type { ResetPasswordInput } from "@/modules/auth/lib/schemas";
import { apiClient } from "@/shared/lib/api-client";

export function useResetPassword() {
	return useMutation({
		mutationFn: (input: ResetPasswordInput) =>
			apiClient.post<{ status: string }>("/api/auth/reset-password", input),
	});
}
