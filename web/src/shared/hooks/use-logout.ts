import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useAppDispatch } from "@/app/hooks";
import { apiClient } from "@/shared/lib/api-client";
import { sessionResolved } from "@/shared/store/auth-slice";

export function useLogout() {
	const dispatch = useAppDispatch();
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: () => apiClient.post("/api/auth/logout"),
		onSuccess: () => {
			dispatch(sessionResolved(null));
			queryClient.setQueryData(["session"], null);
		},
	});
}
