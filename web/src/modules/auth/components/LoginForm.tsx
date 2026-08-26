import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
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
import { useLogin } from "@/modules/auth/hooks/use-login";
import { type LoginInput, loginSchema } from "@/modules/auth/lib/schemas";
import { ApiError } from "@/shared/lib/api-client";

// Does not navigate on success: LoginPage's own <Navigate> guard redirects
// once the session in Redux updates. An imperative navigate() here raced
// against that guard (both fire from the same dispatch) and could land back
// on "/" instead of the intended destination.
export function LoginForm({
	onForgotPassword,
}: {
	onForgotPassword: () => void;
}) {
	const login = useLogin();
	const form = useForm<LoginInput>({
		resolver: zodResolver(loginSchema),
		defaultValues: { email: "", password: "" },
	});

	function onSubmit(values: LoginInput) {
		login.mutate(values, {
			onError: (err) => {
				toast.error(
					err instanceof ApiError ? err.message : "Não foi possível entrar",
				);
			},
		});
	}

	return (
		<Form {...form}>
			<form
				onSubmit={form.handleSubmit(onSubmit)}
				className="flex flex-col gap-4"
			>
				<FormField
					control={form.control}
					name="email"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Email</FormLabel>
							<FormControl>
								<Input
									type="email"
									placeholder="email@exemplo.com"
									autoComplete="email"
									{...field}
								/>
							</FormControl>
							<FormMessage />
						</FormItem>
					)}
				/>
				<FormField
					control={form.control}
					name="password"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Senha</FormLabel>
							<FormControl>
								<Input
									type="password"
									placeholder="••••••••"
									autoComplete="current-password"
									{...field}
								/>
							</FormControl>
							<FormMessage />
						</FormItem>
					)}
				/>
				<Button
					type="button"
					variant="link"
					onClick={onForgotPassword}
					className="h-auto self-end p-0 text-sm text-muted-foreground hover:text-foreground"
				>
					Esqueceu a senha?
				</Button>
				<Button type="submit" disabled={login.isPending} className="mt-1">
					{login.isPending ? "Entrando..." : "Entrar"}
				</Button>
			</form>
		</Form>
	);
}
