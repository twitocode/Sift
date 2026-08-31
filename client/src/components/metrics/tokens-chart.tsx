export type TokenStats = {
	postings_count: number;
	scan_time: number;
};

const formatNumber = (n: number) => n.toLocaleString();

const formatMicros = (micros: number) =>
	micros >= 1000
		? `${(micros / 1000).toFixed(2)} ms`
		: `${micros.toFixed(2)} µs`;

export function TokensChart({
	tokenStats,
}: {
	tokenStats: Record<string, TokenStats>;
}) {
	const data = Object.entries(tokenStats)
		.map(([token, stats]) => ({ token, ...stats }))
		.filter((entry) => entry.postings_count > 0)
		.sort((a, b) => b.postings_count - a.postings_count);

	if (data.length === 0) {
		return null;
	}

	const max = data[0].postings_count;

	return (
		<div className="flex flex-col gap-1.5">
			{data.map((entry) => (
				<div
					key={entry.token}
					className="group relative flex min-h-8 items-center overflow-hidden rounded-md bg-black/5 py-1.5"
					title={`Scan time: ${formatMicros(entry.scan_time)}`}
				>
					<div
						className="absolute inset-y-0 left-0 rounded-md bg-[#FEA93C] transition-[width]"
						style={{
							width: `${Math.max((entry.postings_count / max) * 100, 1)}%`,
						}}
					/>
					<span className="relative z-10 min-w-0 flex-1 break-words pl-2.5 pr-2 text-xs font-bold text-neutral-950">
						{entry.token}
					</span>
					<span className="relative z-10 shrink-0 pr-2.5 font-mono text-xs tabular-nums text-foreground">
						{formatNumber(entry.postings_count)}
					</span>
				</div>
			))}
		</div>
	);
}
