import { useSearchParams } from "react-router-dom";
import { ResetPasswordForm } from "@/modules/auth/components/ResetPasswordForm";

export function ResetPasswordPage() {
	const [params] = useSearchParams();
	const token = params.get("token") ?? "";

	return (
		<div className="flex min-h-svh items-center justify-center bg-background px-6 py-10">
			<div className="flex w-full max-w-[380px] flex-col gap-8">
				<div className="flex flex-col items-center gap-2 text-center">
					<span className="text-[20px] font-bold">Redefinir senha</span>
				</div>
				{token ? (
					<ResetPasswordForm token={token} />
				) : (
					<p className="text-center text-sm text-muted-foreground">
						Link inválido: token ausente.
					</p>
				)}
			</div>
		</div>
	);
}
