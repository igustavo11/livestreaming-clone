import { QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { Provider as ReduxProvider } from "react-redux";
import { queryClient } from "@/app/query-client";
import { store } from "@/app/store";
import { Toaster } from "@/components/ui/sonner";
import { useSession } from "@/shared/hooks/use-session";

// Fetches the session exactly once at the app root; everywhere else reads it
// back from Redux via shared/hooks/use-auth.ts instead of refetching.
function AuthBoot({ children }: { children: ReactNode }) {
	useSession();
	return children;
}

export function AppProviders({ children }: { children: ReactNode }) {
	return (
		<ReduxProvider store={store}>
			<QueryClientProvider client={queryClient}>
				<AuthBoot>{children}</AuthBoot>
				<Toaster />
			</QueryClientProvider>
		</ReduxProvider>
	);
}
