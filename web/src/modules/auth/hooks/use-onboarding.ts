import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useAppDispatch } from "@/app/hooks";
import type { OnboardingInput } from "@/modules/auth/lib/schemas";
import { apiClient } from "@/shared/lib/api-client";
import { AuthResponseSchema } from "@/shared/schemas/channel";
import { sessionResolved } from "@/shared/store/auth-slice";

export function useOnboarding() {
	const dispatch = useAppDispatch();
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: async (input: OnboardingInput) => {
			const data = await apiClient.post("/api/auth/google/onboarding", input);
			return AuthResponseSchema.parse(data);
		},
		onSuccess: (session) => {
			dispatch(sessionResolved(session));
			queryClient.setQueryData(["session"], session);
		},
	});
}
