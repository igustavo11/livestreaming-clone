import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useAppDispatch } from "@/app/hooks";
import type { LoginInput } from "@/modules/auth/lib/schemas";
import { apiClient } from "@/shared/lib/api-client";
import { AuthResponseSchema } from "@/shared/schemas/channel";
import { sessionResolved } from "@/shared/store/auth-slice";

export function useLogin() {
	const dispatch = useAppDispatch();
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: async (input: LoginInput) => {
			const data = await apiClient.post("/api/auth/login", input);
			return AuthResponseSchema.parse(data);
		},
		onSuccess: (session) => {
			dispatch(sessionResolved(session));
			queryClient.setQueryData(["session"], session);
		},
	});
}
