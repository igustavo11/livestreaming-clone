import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { Navigate } from "react-router-dom";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
	Form,
	FormControl,
	FormField,
	FormItem,
	FormLabel,
	FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { useOnboarding } from "@/modules/auth/hooks/use-onboarding";
import {
	type OnboardingInput,
	onboardingSchema,
} from "@/modules/auth/lib/schemas";
import { useAuth } from "@/shared/hooks/use-auth";
import { ApiError } from "@/shared/lib/api-client";

export function OnboardingPage() {
	// Redirect is declarative (reacting to the session Redux now holds),
	// rather than an imperative navigate() in onSuccess — the latter raced
	// against the same dispatch and could leave the URL stuck on this page.
	const { session, isLoading } = useAuth();
	const onboarding = useOnboarding();
	const form = useForm<OnboardingInput>({
		resolver: zodResolver(onboardingSchema),
		defaultValues: { username: "" },
	});

	if (!isLoading && session) {
		return <Navigate to="/dashboard" replace />;
	}

	function onSubmit(values: OnboardingInput) {
		onboarding.mutate(values, {
			onError: (err) => {
				toast.error(
					err instanceof ApiError
						? err.message
						: "Não foi possível concluir o cadastro",
				);
			},
		});
	}

	return (
		<div className="flex min-h-svh items-center justify-center bg-background px-6 py-10">
			<div className="flex w-full max-w-[380px] flex-col gap-8">
				<div className="flex flex-col items-center gap-2 text-center">
					<span className="text-[20px] font-bold">
						Escolha seu nome de usuário
					</span>
					<p className="text-sm text-muted-foreground">
						Essa será a URL do seu canal.
					</p>
				</div>
				<Form {...form}>
					<form
						onSubmit={form.handleSubmit(onSubmit)}
						className="flex flex-col gap-4"
					>
						<FormField
							control={form.control}
							name="username"
							render={({ field }) => (
								<FormItem>
									<FormLabel>Nome de usuário</FormLabel>
									<FormControl>
										<Input
											placeholder="seu_usuario"
											autoComplete="username"
											{...field}
										/>
									</FormControl>
									<FormMessage />
								</FormItem>
							)}
						/>
						<Button type="submit" disabled={onboarding.isPending}>
							{onboarding.isPending ? "Concluindo..." : "Concluir cadastro"}
						</Button>
					</form>
				</Form>
			</div>
		</div>
	);
}
