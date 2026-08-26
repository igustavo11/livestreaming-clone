import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
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
import { useForgotPassword } from "@/modules/auth/hooks/use-forgot-password";
import {
	type ForgotPasswordInput,
	forgotPasswordSchema,
} from "@/modules/auth/lib/schemas";

export function ForgotPasswordForm({ onBack }: { onBack: () => void }) {
	const forgotPassword = useForgotPassword();
	const form = useForm<ForgotPasswordInput>({
		resolver: zodResolver(forgotPasswordSchema),
		defaultValues: { email: "" },
	});

	function onSubmit(values: ForgotPasswordInput) {
		forgotPassword.mutate(values);
	}

	if (forgotPassword.isSuccess) {
		return (
			<div className="flex flex-col gap-4 text-center">
				<p className="text-sm text-muted-foreground">
					Se esse email estiver cadastrado, enviamos um link de redefinição de
					senha.
				</p>
				<Button type="button" variant="outline" onClick={onBack}>
					Voltar para o login
				</Button>
			</div>
		);
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
				<Button type="submit" disabled={forgotPassword.isPending}>
					{forgotPassword.isPending
						? "Enviando..."
						: "Enviar link de redefinição"}
				</Button>
				<Button
					type="button"
					variant="link"
					onClick={onBack}
					className="h-auto p-0 text-sm text-muted-foreground hover:text-foreground"
				>
					Voltar para o login
				</Button>
			</form>
		</Form>
	);
}
