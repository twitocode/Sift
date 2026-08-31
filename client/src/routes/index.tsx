import { createFileRoute } from "@tanstack/react-router";
import Logo from "#/components/logo.tsx";
import SearchBar from "#/components/search-bar.tsx";

export const Route = createFileRoute("/")({
	component: Home,
	head: ({}) => ({
		meta: [
			{
				title: "Sift",
			},
		],
	}),
});

function Home() {
	return (
		<div className="flex w-full flex-1 flex-col items-center justify-center gap-8 md:gap-20">
			<Logo />
			<SearchBar balanced />
		</div>
	);
}
