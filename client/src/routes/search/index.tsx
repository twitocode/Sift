import Logo from "#/components/logo.tsx";
import SearchBar from "#/components/search-bar.tsx";
import SearchResult from "#/components/search-result.tsx";
import { env } from "#/env.ts";
import { createFileRoute } from "@tanstack/react-router";

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
  loader: ({ deps: { query } }) => {
    return getSearchResults(query);
  },
});

type SearchResult = {
  title: string;
  desc: string;
  favicon: string;
  url: string;
};
type SearchResponse = {
  results: SearchResult[];
  meta: {
    success: boolean;
  };
};

async function getSearchResults(query: string): Promise<SearchResponse> {
  const res = await fetch(`${env.VITE_SERVER_URL}/search/${query}`);

  if (!res.ok) {
    return {
      results: [],
      meta: {
        success: false,
      },
    };
  }

  const data = await res.json();
  return {
    results: data,
    meta: {
      success: true,
    },
  };
}

function Home() {
  const { query } = Route.useSearch();
  const data = Route.useLoaderData();

  return (
    <div className="">
      <div className="flex gap-2 w-full mb-4">
        <Logo noText />
        <SearchBar initial={query} />
      </div>
      <div></div>
      <section className="mt-5 border-t-gray-500 border-t pt-2">
        {data.results?.map((x) => (
          <SearchResult
            desc={x.desc}
            favicon={x.favicon}
            url={x.url}
            title={x.title}
            key={x.url}
          />
        ))}
      </section>
    </div>
  );
}
