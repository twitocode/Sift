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

const formatMicros = (micros: number) =>
  micros >= 1000
    ? `${(micros / 1000).toFixed(2)} ms`
    : `${micros.toFixed(2)} µs`;

export default function SearchMetrics(props: SearchMetricsProps) {
  const hasTokens = Object.keys(props.tokenStats).length > 0;

  return (
    <aside className="md:pt-2 px-6  flex flex-col gap-10 self-start border-t border-t-gray-500 py-4 md:sticky md:top-5 md:mt-5 md:max-h-[calc(100vh-2.5rem)] md:overflow-y-auto md:border-t-0 md:border-l  md:pl-6">
      <div className="flex flex-col gap-6">
        <div>
          <h2 className="text-xl font-bold">Query metrics</h2>
          <p className="text-sm text-gray-600">
            {/* {formatNumber(props.count)}*/} Queried in {props.timeElapsed} ms 
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
            <h3 className="mb-2 text-sm font-bold tracking-wide text-gray-500">
              Pages Containing Token
            </h3>
            <TokensChart tokenStats={props.tokenStats} />
          </div>
        )}
      </div>

      <div className="border-t-gray-600 border-2 pt-10">
        <h2 className="mb-2 text-xl font-bold tracking-wide">Index Metrics</h2>
        <div className="grid grid-cols-2 gap-2">
          <Stat
            label="Docs indexed"
            value={formatNumber(props.indexMetrics.docs_indexed)}
          />

          <Stat
            label="Unique terms"
            value={formatNumber(props.indexMetrics.unique_terms)}
          />
          <Stat
            label="Total postings"
            value={formatNumber(props.indexMetrics.total_postings)}
          />
          <Stat
            label="Body tokens"
            value={formatNumber(props.indexMetrics.body_tokens)}
          />
          <Stat
            label="Title tokens"
            value={formatNumber(props.indexMetrics.title_tokens)}
          />
        </div>
      </div>
    </aside>
  );
}
