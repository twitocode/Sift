import Logo from "#/components/logo.tsx";
import SearchMetrics from "#/components/metrics/search-metrics.tsx";
import { ModeToggle } from "#/components/mode-toggle.tsx";
import SearchBar from "#/components/search-bar.tsx";
import SearchResult from "#/components/search-result.tsx";
import { getSearchResults } from "#/lib/server.ts";
import { tokensToIgnore } from "#/lib/tokens.ts";
import { cn } from "#/lib/utils.ts";
import { createFileRoute } from "@tanstack/react-router";
import { ArrowDown } from "lucide-react";
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
    const data = await getSearchResults(query);

    data.token_stats = Object.fromEntries(
      Object.entries(data.token_stats).filter(([token]) => {
        return !tokensToIgnore.has(token);
      }),
    );

    return data
  },
  head: ({ match }) => ({
    meta: [
      {
        title: `${match.search.query} - Sift`,
      },
    ],
  }),
});

function Home() {
  const { query } = Route.useSearch();
  const data = Route.useLoaderData();

  const [isScrolled, setIsScrolled] = useState(false);
  const [isNearBottom, setIsNearBottom] = useState(false);

  useEffect(() => {
    const handleScroll = () => {
      setIsScrolled(window.scrollY > 10);
      setIsNearBottom(
        window.innerHeight + window.scrollY >=
          document.documentElement.scrollHeight - 100,
      );
    };
    handleScroll();
    window.addEventListener("scroll", handleScroll);
    return () => window.removeEventListener("scroll", handleScroll);
  }, []);

  return (
    <>
    <main className="grid w-full min-w-0 flex-1 grid-cols-[auto_minmax(0,1fr)_auto] lg:grid-cols-[auto_minmax(0,1fr)_minmax(0,1fr)]">
      <header
        className={cn(
          "col-span-3 mb-4 grid grid-cols-subgrid items-center gap-x-3 sticky top-0 z-50 bg-background py-2 transition-shadow duration-75 ease-in lg:static lg:gap-x-0",
          isScrolled ? "shadow-md lg:shadow-none" : "shadow-none",
        )}
      >
        <div className="shrink-0 lg:pr-4">
          <Logo noText />
        </div>
        <div className="min-w-0">
          <SearchBar initial={query} />
        </div>
        <div className="flex justify-end lg:justify-start lg:pl-6">
          <ModeToggle />
        </div>
      </header>
      <section className="col-span-3 min-w-0 border-t border-border md:mt-5 md:pt-2 lg:col-span-1 lg:col-start-2 lg:border-t-0">
        {data.results?.map((x, i) => (
          <SearchResult
            desc={x.desc}
            score={x.score}
            favicon={x.favicon}
            url={x.url}
            title={x.title}
            key={x.url + i}
            ogTitle={x.og_title}
            titleTokens={x.title_tokens}
            bodyTokens={x.body_tokens}
          />
        ))}
      </section>
      <div className="col-span-3 min-w-0 lg:col-span-1">
        <SearchMetrics
          count={data.count}
          timeElapsed={data.time_elapsed}
          averagePostingsScanDuration={data.average_postings_scan_duration}
          tokenStats={data.token_stats}
          possibleResults={data.possible_results}
          indexMetrics={data.index_metrics}
        />
      </div>
    </main>
    <button
      type="button"
      aria-label="Scroll to bottom"
      onClick={() =>
        window.scrollTo({
          top: document.documentElement.scrollHeight,
          behavior: "smooth",
        })
      }
      className={cn(
        "fixed bottom-6 right-6 z-50 flex size-auto items-center justify-center rounded-full bg-foreground text-background shadow-lg transition-opacity duration-200 lg:hidden",
        isNearBottom ? "pointer-events-none opacity-0" : "opacity-100",
      )}
    >
      <div className="flex gap-1 px-5 py-3  items-center ">
        <ArrowDown className="size-5 " />
        <span className="">Query Metrics</span>
      </div>
    </button>
    </>
  );
}
