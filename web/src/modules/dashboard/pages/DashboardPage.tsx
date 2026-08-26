import { Skeleton } from "@/components/ui/skeleton";
import { AvatarUploader } from "@/modules/dashboard/components/AvatarUploader";
import { LiveInfoForm } from "@/modules/dashboard/components/LiveInfoForm";
import { LiveStatusBanner } from "@/modules/dashboard/components/LiveStatusBanner";
import { StreamKeyCard } from "@/modules/dashboard/components/StreamKeyCard";
import { useDashboardChannel } from "@/modules/dashboard/hooks/use-dashboard-channel";
import { StatusPill } from "@/shared/components/channel/StatusPill";
import { AppHeader } from "@/shared/components/layout/AppHeader";

export function DashboardPage() {
	const dashboardChannel = useDashboardChannel();

	return (
		<div className="min-h-svh bg-background">
			<AppHeader breadcrumb="Dashboard" />

			<div className="mx-auto flex max-w-[920px] flex-col gap-8 px-6 py-12 md:px-8">
				{dashboardChannel.data ? (
					<>
						<div className="flex items-center justify-between">
							<div className="flex items-center gap-4">
								<AvatarUploader
									username={dashboardChannel.data.username}
									avatarUrl={dashboardChannel.data.avatar_url}
								/>
								<div>
									<h1 className="text-2xl font-bold">
										{dashboardChannel.data.username}
									</h1>
									<p className="mt-1 text-sm text-muted-foreground">
										Gerencie sua live e configurações do canal
									</p>
								</div>
							</div>
							<StatusPill live={dashboardChannel.data.is_live} />
						</div>

						<StreamKeyCard preview={dashboardChannel.data.stream_key_preview} />
						<LiveInfoForm channel={dashboardChannel.data} />
						<LiveStatusBanner live={dashboardChannel.data.is_live} />
					</>
				) : (
					<div className="flex flex-col gap-8">
						<Skeleton className="h-9 w-48" />
						<Skeleton className="h-48 w-full rounded-2xl" />
						<Skeleton className="h-96 w-full rounded-2xl" />
					</div>
				)}
			</div>
		</div>
	);
}
