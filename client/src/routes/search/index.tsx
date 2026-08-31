import { createFileRoute } from "@tanstack/react-router";
import { ArrowDown } from "lucide-react";
import { useEffect, useState } from "react";
import Logo from "#/components/logo.tsx";
import SearchMetrics from "#/components/metrics/search-metrics.tsx";
import SearchBar from "#/components/search-bar.tsx";
import SearchResult from "#/components/search-result.tsx";
import { getSearchResults } from "#/lib/server.ts";
import { cn } from "#/lib/utils.ts";

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
		<main className="max-w-375">
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
			<div className="grid lg:grid-cols-2">
				<section className="border-t border-border px-2 md:mt-5 md:pt-2 lg:border-t-0">
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

				<SearchMetrics
					count={data.count}
					timeElapsed={data.time_elapsed}
					averagePostingsScanDuration={data.average_postings_scan_duration}
					tokenStats={data.token_stats}
					possibleResults={data.possible_results}
					indexMetrics={data.index_metrics}
				/>
			</div>
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
		</main>
	);
}
