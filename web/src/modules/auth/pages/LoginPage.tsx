import { useState } from "react";
import { Navigate } from "react-router-dom";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ForgotPasswordForm } from "@/modules/auth/components/ForgotPasswordForm";
import { GoogleButton } from "@/modules/auth/components/GoogleButton";
import { LoginForm } from "@/modules/auth/components/LoginForm";
import { SignupForm } from "@/modules/auth/components/SignupForm";
import { useAuth } from "@/shared/hooks/use-auth";

export function LoginPage() {
	const { session, isLoading } = useAuth();
	const [tab, setTab] = useState<"login" | "signup">("login");
	const [forgotPassword, setForgotPassword] = useState(false);
	const [justSignedUp, setJustSignedUp] = useState(false);

	if (!isLoading && session) {
		return <Navigate to={justSignedUp ? "/dashboard" : "/"} replace />;
	}

	return (
		<div className="flex min-h-svh items-center justify-center bg-background px-6 py-10">
			<div className="flex w-full max-w-[380px] flex-col gap-8">
				<div className="flex items-center justify-center gap-3">
					<span className="relative flex size-7 items-center justify-center rounded-lg bg-primary">
						<span className="absolute top-2 left-2 size-3 rounded-full bg-background" />
					</span>
					<span className="text-[17px] font-bold">Ao Vivo</span>
				</div>

				{forgotPassword ? (
					<ForgotPasswordForm onBack={() => setForgotPassword(false)} />
				) : (
					<>
						<Tabs
							value={tab}
							onValueChange={(value) => setTab(value as "login" | "signup")}
						>
							<TabsList className="w-full">
								<TabsTrigger value="login" className="flex-1">
									Entrar
								</TabsTrigger>
								<TabsTrigger value="signup" className="flex-1">
									Criar conta
								</TabsTrigger>
							</TabsList>
							<TabsContent value="login" className="mt-6">
								<LoginForm onForgotPassword={() => setForgotPassword(true)} />
							</TabsContent>
							<TabsContent value="signup" className="mt-6">
								<SignupForm onSignedUp={() => setJustSignedUp(true)} />
							</TabsContent>
						</Tabs>

						<div className="flex items-center gap-3 text-xs text-muted-foreground">
							<Separator className="flex-1" />
							ou
							<Separator className="flex-1" />
						</div>

						<GoogleButton />
					</>
				)}
			</div>
		</div>
	);
}
