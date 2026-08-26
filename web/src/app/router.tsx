import type { ReactNode } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { LoginPage } from "@/modules/auth/pages/LoginPage";
import { OnboardingPage } from "@/modules/auth/pages/OnboardingPage";
import { ResetPasswordPage } from "@/modules/auth/pages/ResetPasswordPage";
import { ChannelPage } from "@/modules/channel/pages/ChannelPage";
import { DashboardPage } from "@/modules/dashboard/pages/DashboardPage";
import { HomePage } from "@/modules/home/pages/HomePage";
import { useAuth } from "@/shared/hooks/use-auth";

function ProtectedRoute({ children }: { children: ReactNode }) {
	const { session, isLoading } = useAuth();
	if (isLoading) return null;
	if (!session) return <Navigate to="/login" replace />;
	return children;
}

export function AppRouter() {
	return (
		<BrowserRouter>
			<Routes>
				<Route path="/" element={<HomePage />} />
				<Route path="/login" element={<LoginPage />} />
				<Route path="/onboarding" element={<OnboardingPage />} />
				<Route path="/reset-password" element={<ResetPasswordPage />} />
				<Route
					path="/dashboard"
					element={
						<ProtectedRoute>
							<DashboardPage />
						</ProtectedRoute>
					}
				/>
				{/* Public channel pages are a catch-all keyed by username — register last. */}
				<Route path="/:username" element={<ChannelPage />} />
			</Routes>
		</BrowserRouter>
	);
}
