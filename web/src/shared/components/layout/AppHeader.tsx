import { LayoutDashboard, LogOut } from "lucide-react";
import type { ReactNode } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import { ChannelAvatar } from "@/shared/components/channel/ChannelAvatar";
import { useAuth } from "@/shared/hooks/use-auth";
import { useLogout } from "@/shared/hooks/use-logout";

type AppHeaderProps = {
	center?: ReactNode;
	breadcrumb?: string;
	className?: string;
};

export function AppHeader({ center, breadcrumb, className }: AppHeaderProps) {
	const { session } = useAuth();
	const logout = useLogout();
	const navigate = useNavigate();

	return (
		<header
			className={cn(
				"sticky top-0 z-10 flex items-center justify-between gap-4 border-b border-border bg-background px-6 py-4 md:px-10",
				className,
			)}
		>
			<Link to="/" className="flex shrink-0 items-center gap-3">
				<span className="relative flex size-7 items-center justify-center rounded-lg bg-primary">
					<span className="absolute top-2 left-2 size-3 rounded-full bg-background" />
				</span>
				<span className="text-[17px] font-bold tracking-tight">Ao Vivo</span>
				{breadcrumb ? (
					<span className="ml-1 text-sm text-muted-foreground">
						/ {breadcrumb}
					</span>
				) : null}
			</Link>

			{center ? (
				<div className="hidden flex-1 justify-center md:flex">{center}</div>
			) : null}

			<div className="flex shrink-0 items-center gap-4">
				{session ? (
					<DropdownMenu>
						<DropdownMenuTrigger asChild>
							<Button
								variant="ghost"
								size="icon"
								aria-label="Menu da conta"
								className="rounded-full"
							>
								<ChannelAvatar
									username={session.channel.username}
									avatarUrl={session.channel.avatar_url}
								/>
							</Button>
						</DropdownMenuTrigger>
						<DropdownMenuContent align="end">
							<DropdownMenuItem onSelect={() => navigate("/dashboard")}>
								<LayoutDashboard /> Dashboard
							</DropdownMenuItem>
							<DropdownMenuSeparator />
							<DropdownMenuItem onSelect={() => logout.mutate()}>
								<LogOut /> Sair
							</DropdownMenuItem>
						</DropdownMenuContent>
					</DropdownMenu>
				) : (
					<Button onClick={() => navigate("/login")}>Entrar</Button>
				)}
			</div>
		</header>
	);
}
