import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { useNavigate } from "react-router-dom";
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
import { useResetPassword } from "@/modules/auth/hooks/use-reset-password";
import {
	type ResetPasswordInput,
	resetPasswordSchema,
} from "@/modules/auth/lib/schemas";
import { ApiError } from "@/shared/lib/api-client";

export function ResetPasswordForm({ token }: { token: string }) {
	const navigate = useNavigate();
	const resetPassword = useResetPassword();
	const form = useForm<ResetPasswordInput>({
		resolver: zodResolver(resetPasswordSchema),
		defaultValues: { token, password: "" },
	});

	function onSubmit(values: ResetPasswordInput) {
		resetPassword.mutate(values, {
			onSuccess: () => {
				toast.success("Senha redefinida. Faça login com a nova senha.");
				navigate("/login");
			},
			onError: (err) => {
				toast.error(
					err instanceof ApiError ? err.message : "Link inválido ou expirado",
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
					name="password"
					render={({ field }) => (
						<FormItem>
							<FormLabel>Nova senha</FormLabel>
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
				<Button type="submit" disabled={resetPassword.isPending}>
					{resetPassword.isPending ? "Salvando..." : "Redefinir senha"}
				</Button>
			</form>
		</Form>
	);
}
