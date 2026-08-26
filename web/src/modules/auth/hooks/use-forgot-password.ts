import { useMutation } from "@tanstack/react-query";
import type { ForgotPasswordInput } from "@/modules/auth/lib/schemas";
import { apiClient } from "@/shared/lib/api-client";

export function useForgotPassword() {
	return useMutation({
		mutationFn: (input: ForgotPasswordInput) =>
			apiClient.post<{ status: string }>("/api/auth/forgot-password", input),
	});
}
