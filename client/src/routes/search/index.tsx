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
};

type SearchResponse = {
  results: SearchResult[];
  query: string;
  meta: {
    success: boolean;
  };
};

async function getSearchResults(query: string): Promise<SearchResponse> {
  const res = await fetch(`${env.VITE_SERVER_URL}/search/${query}`);

  if (!res.ok) {
    return {
      results: [],
      query,
      meta: {
        success: false,
      },
    };
  }

  const data = await res.json();
  return {
    results: data,
    query,
    meta: {
      success: true,
    },
  };
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
