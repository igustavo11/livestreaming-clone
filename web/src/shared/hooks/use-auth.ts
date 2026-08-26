import { useAppSelector } from "@/app/hooks";

// Cheap read of the session Redux already holds — does not trigger a fetch.
// Use shared/hooks/use-session.ts (mounted once at the app root) for that.
export function useAuth() {
	const session = useAppSelector((state) => state.auth.session);
	const status = useAppSelector((state) => state.auth.status);
	return { session, isLoading: status === "loading" };
}
