export function Stat({ label, value }: { label: string; value: string }) {
	return (
		<div className="flex flex-col gap-1 rounded-lg bg-card/60 p-3">
			<span className="text-xs tracking-wide text-muted-foreground">
				{label}
			</span>
			<span className="text-lg font-bold text-foreground">{value}</span>
		</div>
	);
}
