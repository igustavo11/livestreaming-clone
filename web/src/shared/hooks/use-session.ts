import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import { useAppDispatch } from "@/app/hooks";
import { ApiError, apiClient } from "@/shared/lib/api-client";
import { AuthResponseSchema } from "@/shared/schemas/channel";
import { type Session, sessionResolved } from "@/shared/store/auth-slice";

// Fetches /api/auth/me once and mirrors the result into Redux. Mount this
// exactly once near the app root (see app/providers.tsx); everywhere else
// should read the session via shared/hooks/use-auth.ts instead of refetching.
export function useSession() {
	const dispatch = useAppDispatch();

	const query = useQuery({
		queryKey: ["session"],
		queryFn: async (): Promise<Session> => {
			try {
				const data = await apiClient.get("/api/auth/me");
				return AuthResponseSchema.parse(data);
			} catch (err) {
				if (err instanceof ApiError && err.status === 401) {
					return null;
				}
				throw err;
			}
		},
	});

	useEffect(() => {
		if (query.isSuccess) {
			dispatch(sessionResolved(query.data));
		}
	}, [query.isSuccess, query.data, dispatch]);

	return query;
}
