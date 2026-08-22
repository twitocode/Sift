import Logo from "#/components/logo.tsx";
import SearchBar from "#/components/search-bar.tsx";
import SearchResult from "#/components/search-result.tsx";
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
  meta: {};
};

async function getSearchResults(query: string): Promise<SearchResponse> {
  const results = [
    {
      title: "TanStack Router — Type-safe Routing for React",
      desc: "Fully type-safe, modern routing for React and React Native applications. Nested routes, search params, and loaders without the boilerplate.",
      favicon: "https://www.google.com/s2/favicons?domain=tanstack.com&sz=32",
      url: "https://tanstack.com/router/latest",
    },
    {
      title: "MDN Web Docs: Fetch API",
      desc: "The Fetch API provides a JavaScript interface for accessing and manipulating parts of the HTTP pipeline, such as requests and responses.",
      favicon:
        "https://www.google.com/s2/favicons?domain=developer.mozilla.org&sz=32",
      url: "https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API",
    },
    {
      title: "Wikipedia — Information retrieval",
      desc: "Information retrieval is the process of obtaining information system resources that are relevant to an information need from a collection of those resources.",
      favicon:
        "https://www.google.com/s2/favicons?domain=en.wikipedia.org&sz=32",
      url: "https://en.wikipedia.org/wiki/Information_retrieval",
    },
    {
      title: "GitHub: vercel/next.js",
      desc: "The React Framework for the Web. Used by some of the world's largest companies, Next.js enables you to create full-stack web applications.",
      favicon: "https://www.google.com/s2/favicons?domain=github.com&sz=32",
      url: "https://github.com/vercel/next.js",
    },
    {
      title: "CSS-Tricks — A Complete Guide to Flexbox",
      desc: "Our comprehensive guide to CSS flexbox layout. This complete guide explains everything about flexbox, focusing on all the different possible properties.",
      favicon: "https://www.google.com/s2/favicons?domain=css-tricks.com&sz=32",
      url: "https://css-tricks.com/snippets/css/a-guide-to-flexbox/",
    },
  ];

  return {
    results,
    meta: {},
  };
}

function Home() {
  const { query } = Route.useSearch();
  const data = Route.useLoaderData();

  return (
    <>
      <div className="flex gap-2">
        <Logo noText />
        <SearchBar initial={query} />
      </div>
      <section className="mt-5">
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
    </>
  );
}
