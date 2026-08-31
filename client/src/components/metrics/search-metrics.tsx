import { Stat } from "#/components/metrics/metrics.stat.tsx";
import {
	type TokenStats,
	TokensChart,
} from "#/components/metrics/tokens-chart.tsx";

export type { TokenStats };

export type IndexMetrics = {
	docs_read: number;
	docs_indexed: number;
	body_tokens: number;
	title_tokens: number;
	unique_terms: number;
	total_postings: number;
	title_postings: number;
	time_elapsed: number;
};

type SearchMetricsProps = {
	count: number;
	timeElapsed: number;
	averagePostingsScanDuration: number;
	tokenStats: Record<string, TokenStats>;
	possibleResults: number;
	indexMetrics: IndexMetrics;
};

const formatNumber = (n: number) => n.toLocaleString();

const formatCompact = (n: number) =>
	n.toLocaleString("en-US", {
		notation: "compact",
		maximumFractionDigits: 1,
	});

function IndexStat({ label, value }: { label: string; value: number }) {
	return (
		<div className="flex items-baseline justify-between gap-4 py-2">
			<span className="text-sm text-muted-foreground">{label}</span>
			<span
				className="font-mono text-sm font-bold tabular-nums text-foreground"
				title={formatNumber(value)}
			>
				{formatCompact(value)}
			</span>
		</div>
	);
}

const formatMicros = (micros: number) =>
	micros >= 1000
		? `${(micros / 1000).toFixed(2)} ms`
		: `${micros.toFixed(2)} µs`;

export default function SearchMetrics(props: SearchMetricsProps) {
	const hasTokens = Object.keys(props.tokenStats).length > 0;

	return (
		<aside className="flex flex-col gap-10 self-start border-t border-border px-6 py-4 md:sticky md:top-5 md:mt-5 md:max-h-[calc(100vh-2.5rem)] md:overflow-y-auto md:border-t-0 md:border-l md:pl-6">
			<div className="flex flex-col gap-6">
				<div>
					<h2 className="text-xl font-bold">Query metrics</h2>
					<p className="text-sm text-muted-foreground">
						{/* {formatNumber(props.count)}*/} Top 50 results queried in{" "}
						{props.timeElapsed} ms
					</p>
				</div>

				<div className="grid grid-cols-2 gap-2">
					<Stat
						label="Possible results"
						value={formatNumber(props.possibleResults)}
					/>
					<Stat
						label="Avg postings scan"
						value={formatMicros(props.averagePostingsScanDuration)}
					/>
				</div>

				{hasTokens && (
					<div>
						<h3 className="mb-2 text-sm font-bold tracking-wide text-muted-foreground">
							Pages Containing Token
						</h3>
						<TokensChart tokenStats={props.tokenStats} />
					</div>
				)}
			</div>

			<div className="flex flex-col gap-3 border-t border-border pt-6">
				<div>
					<h2 className="text-xl font-bold tracking-wide">Index Metrics</h2>
					<p className="mt-1 text-sm text-muted-foreground">
						Postings keep track of which documents contain a search term,
						including the term’s title, body and URL frequency.
					</p>
				</div>
				<div className="flex flex-col divide-y divide-border">
					<IndexStat
						label="Docs indexed"
						value={props.indexMetrics.docs_indexed}
					/>
					<IndexStat
						label="Unique terms"
						value={props.indexMetrics.unique_terms}
					/>
					<IndexStat
						label="Total postings"
						value={props.indexMetrics.total_postings}
					/>
					<IndexStat
						label="Body tokens"
						value={props.indexMetrics.body_tokens}
					/>
					<IndexStat
						label="Title tokens"
						value={props.indexMetrics.title_tokens}
					/>
				</div>
			</div>
		</aside>
	);
}
