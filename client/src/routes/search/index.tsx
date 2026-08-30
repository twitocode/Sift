import Logo from "#/components/logo.tsx";
import SearchBar from "#/components/search-bar.tsx";
import SearchResult from "#/components/search-result.tsx";
import { env } from "#/env.ts";
import { cn } from "#/lib/utils.ts";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";

type QueryParams = {
  query: string;
};

export const Route = createFileRoute("/search/")({
  component: Home,

  validateSearch: (search: Record<string, string>): QueryParams => {
    return {
      query: search.q,
    };
  },
  loaderDeps: ({ search }) => {
    return { query: search.query };
  },
  loader: async ({ deps: { query } }) => {
    return getSearchResults(query);
  },
  head: ({ loaderData }) => ({
    meta: [
      {
        title: loaderData?.query + " - Sift",
      },
    ],
  }),
});

type SearchResult = {
  title: string;
  og_title: string;
  desc: string;
  favicon: string;
  url: string;
  score: number;
};

type TokenStats = {
  postings_count: number;
  scan_time: number;
};

type IndexMetrics = {
  docs_read: number;
  docs_indexed: number;
  body_tokens: number;
  title_tokens: number;
  unique_terms: number;
  total_postings: number;
  title_postings: number;
  time_elapsed: number;
};

type SearchResponse = {
  results: SearchResult[];
  count: number;
  time_elapsed: number;
  average_postings_scan_duration: number;
  token_stats: Record<string, TokenStats>;
  possible_results: number;
  index_metrics: IndexMetrics;
};

async function getSearchResults(query: string): Promise<SearchResponse> {
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

function Home() {
  const { query } = Route.useSearch();
  const data = Route.useLoaderData();

  const [isScrolled, setIsScrolled] = useState(false);

  useEffect(() => {
    const handleScroll = () => {
      setIsScrolled(window.scrollY > 10);
    };
    window.addEventListener("scroll", handleScroll);
    return () => window.removeEventListener("scroll", handleScroll);
  }, []);

  return (
    <div className="">
      <div
        className={cn(
          "flex gap-8 w-full mb-4 bg-background p-2 sticky md:static top-0 z-50 ease-in duration-75 transition-shadow",
          isScrolled ? "shadow-md md:shadow-none" : "shadow-none",
        )}
      >
        <Logo noText />
        <SearchBar initial={query} />
      </div>
      <div></div>
      <section className="md:mt-5 border-t-gray-500 border-t md:pt-2 px-2 ">
        {data.results?.map((x, i) => (
          <SearchResult
            desc={x.desc}
            favicon={x.favicon}
            url={x.url}
            title={x.title}
            key={x.url + i}
            ogTitle={x.og_title}
          />
        ))}
      </section>
    </div>
  );
}
