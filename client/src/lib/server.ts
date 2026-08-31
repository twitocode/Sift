import {
  type IndexMetrics,
  type TokenStats,
} from "#/components/metrics/search-metrics.tsx";
import { env } from "#/env.ts";

export type SearchResultItem = {
  title: string;
  og_title: string;
  desc: string;
  favicon: string;
  url: string;
  original_url: string;
  score: number;
  title_tokens: number;
  body_tokens: number;
};

export type SearchResponse = {
  results: SearchResultItem[];
  count: number;
  time_elapsed: number;
  average_postings_scan_duration: number;
  token_stats: Record<string, TokenStats>;
  possible_results: number;
  index_metrics: IndexMetrics;
};

export async function getSearchResults(query: string): Promise<SearchResponse> {
  const res = await fetch(`${env.VITE_SERVER_URL}/search/${query}`);

  if (!res.ok) {
    return {
      results: [],
      count: 0,
      time_elapsed: 0,
      average_postings_scan_duration: 0,
      token_stats: {},
      possible_results: 0,
      index_metrics: {
        docs_read: 0,
        docs_indexed: 0,
        body_tokens: 0,
        title_tokens: 0,
        unique_terms: 0,
        total_postings: 0,
        title_postings: 0,
        time_elapsed: 0,
      },
    };
  }

  return res.json() as Promise<SearchResponse>;
}
