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
import { useSignup } from "@/modules/auth/hooks/use-signup";
import { type SignupInput, signupSchema } from "@/modules/auth/lib/schemas";
import { ApiError } from "@/shared/lib/api-client";

// Does not navigate itself: calls onSignedUp (a plain state setter) so
// LoginPage's own <Navigate> guard performs the actual redirect once the
// session in Redux updates. An imperative navigate() here raced against that
// guard and could land back on "/" instead of "/dashboard".
export function SignupForm({ onSignedUp }: { onSignedUp: () => void }) {
	const signup = useSignup();
	const form = useForm<SignupInput>({
		resolver: zodResolver(signupSchema),
		defaultValues: { email: "", username: "", password: "" },
	});

	function onSubmit(values: SignupInput) {
		signup.mutate(values, {
			onSuccess: onSignedUp,
			onError: (err) => {
				toast.error(
					err instanceof ApiError
						? err.message
						: "Não foi possível criar a conta",
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
									autoComplete="new-password"
									{...field}
								/>
							</FormControl>
							<FormMessage />
						</FormItem>
					)}
				/>
				<Button type="submit" disabled={signup.isPending} className="mt-1">
					{signup.isPending ? "Criando conta..." : "Criar conta"}
				</Button>
			</form>
		</Form>
	);
}
