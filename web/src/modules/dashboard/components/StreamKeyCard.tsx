import { Copy, RefreshCw } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { useRotateStreamKey } from "@/modules/dashboard/hooks/use-rotate-stream-key";

function copyToClipboard(value: string) {
	void navigator.clipboard.writeText(value);
	toast.success("Copiado");
}

export function StreamKeyCard({ preview }: { preview: string }) {
	const rotateStreamKey = useRotateStreamKey();
	const [confirmOpen, setConfirmOpen] = useState(false);
	const [revealedKey, setRevealedKey] = useState<string | null>(null);

	function handleRotate() {
		rotateStreamKey.mutate(undefined, {
			onSuccess: (data) => {
				setConfirmOpen(false);
				setRevealedKey(data.stream_key);
			},
			onError: () => toast.error("Não foi possível gerar uma nova stream key"),
		});
	}

	return (
		<section className="flex flex-col gap-5 rounded-2xl border border-border bg-card p-7">
			<h2 className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
				Stream key
			</h2>
			<div className="flex flex-col gap-3 sm:flex-row">
				<div className="flex-1 truncate rounded-lg border border-input bg-background px-4 py-3 font-mono text-sm tracking-wide text-muted-foreground">
					{preview || "nenhuma stream key gerada ainda"}
				</div>
				<div className="flex gap-2.5">
					<Button
						type="button"
						variant="secondary"
						onClick={() => copyToClipboard(preview)}
						disabled={!preview}
					>
						<Copy /> Copiar
					</Button>
					<Button
						type="button"
						variant="outline"
						onClick={() => setConfirmOpen(true)}
					>
						<RefreshCw /> Regenerar
					</Button>
				</div>
			</div>
			<p className="text-xs text-muted-foreground">
				Nunca compartilhe sua stream key. Regenerá-la desconecta transmissões em
				andamento.
			</p>

			<Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>Gerar nova stream key?</DialogTitle>
						<DialogDescription>
							A chave atual deixa de funcionar imediatamente e qualquer
							transmissão em andamento será desconectada.
						</DialogDescription>
					</DialogHeader>
					<DialogFooter>
						<Button
							type="button"
							variant="outline"
							onClick={() => setConfirmOpen(false)}
						>
							Cancelar
						</Button>
						<Button
							type="button"
							onClick={handleRotate}
							disabled={rotateStreamKey.isPending}
						>
							{rotateStreamKey.isPending ? "Gerando..." : "Gerar nova chave"}
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			<Dialog
				open={revealedKey !== null}
				onOpenChange={(open) => !open && setRevealedKey(null)}
			>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>Sua nova stream key</DialogTitle>
						<DialogDescription>
							Copie agora — ela não será mostrada novamente.
						</DialogDescription>
					</DialogHeader>
					<div className="flex items-center gap-2.5">
						<code className="flex-1 truncate rounded-lg border border-input bg-background px-4 py-3 text-sm">
							{revealedKey}
						</code>
						<Button
							type="button"
							size="icon"
							onClick={() => revealedKey && copyToClipboard(revealedKey)}
						>
							<Copy />
						</Button>
					</div>
				</DialogContent>
			</Dialog>
		</section>
	);
}
