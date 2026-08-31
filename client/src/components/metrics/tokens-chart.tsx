import { Bar, BarChart, LabelList, XAxis, YAxis } from "recharts";

import {
	type ChartConfig,
	ChartContainer,
	ChartTooltip,
	ChartTooltipContent,
} from "#/components/ui/chart";

export type TokenStats = {
	postings_count: number;
	scan_time: number;
};

const formatNumber = (n: number) => n.toLocaleString();

const formatMicros = (micros: number) =>
	micros >= 1000
		? `${(micros / 1000).toFixed(2)} ms`
		: `${micros.toFixed(2)} µs`;

const chartConfig = {
	postings_count: {
		label: "Pages with token",
		color: "#FEA93C",
	},
} satisfies ChartConfig;

export function TokensChart({
	tokenStats,
}: {
	tokenStats: Record<string, TokenStats>;
}) {
	const data = Object.entries(tokenStats)
		.map(([token, stats]) => ({ token, ...stats }))
		.sort((a, b) => b.postings_count - a.postings_count);

	return (
		<ChartContainer
			config={chartConfig}
			className="aspect-auto w-full overflow-visible [&_.recharts-surface]:overflow-visible [&_.recharts-wrapper]:overflow-visible"
			style={{ height: Math.max(data.length * 40, 80) }}
		>
			<BarChart
				accessibilityLayer
				data={data}
				layout="vertical"
				margin={{ left: 8, right: 48 }}
			>
				<XAxis type="number" dataKey="postings_count" hide />
				<YAxis dataKey="token" type="category" hide />
				<ChartTooltip
					cursor={false}
					content={
						<ChartTooltipContent
							hideLabel
							formatter={(_value, _name, item) => {
								const stats = item.payload as TokenStats & { token: string };
								return (
									<div className="flex flex-col gap-1">
										<span className="font-bold text-foreground">{stats.token}</span>
										<div className="flex flex-1 items-center justify-between gap-4 leading-none">
											<span className="text-muted-foreground">Scan time</span>
											<span className="font-mono font-medium text-foreground tabular-nums">
												{formatMicros(stats.scan_time)}
											</span>
										</div>
									</div>
								);
							}}
						/>
					}
				/>
				<Bar
					dataKey="postings_count"
					fill="var(--color-postings_count)"
					radius={5}
					isAnimationActive={false}
				>
					<LabelList
						dataKey="token"
						position="insideLeft"
						offset={8}
						className="fill-neutral-950 font-bold"
						fontSize={12}
						fontWeight={700}
					/>
					<LabelList
						dataKey="postings_count"
						position="right"
						offset={8}
						className="fill-foreground"
						fontSize={12}
						formatter={(value) => formatNumber(Number(value))}
					/>
				</Bar>
			</BarChart>
		</ChartContainer>
	);
}
