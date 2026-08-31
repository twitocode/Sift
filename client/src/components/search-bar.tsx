import { useNavigate } from "@tanstack/react-router";
import { SearchIcon } from "lucide-react";
import { useState } from "react";
import { ModeToggle } from "#/components/mode-toggle.tsx";
import {
	InputGroup,
	InputGroupAddon,
	InputGroupInput,
} from "#/components/ui/input-group.tsx";
import { cn } from "#/lib/utils.ts";

type SearchBarProps = {
	initial?: string;
	balanced?: boolean;
};
export default function SearchBar(props: SearchBarProps) {
	const [query, setQuery] = useState(props.initial ?? "");
	const navigate = useNavigate();

	const handleSearch = (
		e: React.ChangeEvent<HTMLFormElement, HTMLFormElement>,
	) => {
		e.preventDefault();
		navigate({ to: "/search" + `?q=${query}` });
	};

	return (
		<div
			className={cn(
				"flex w-full items-center gap-2",
				props.balanced ? "max-w-xl" : "md:w-max",
			)}
		>
			{props.balanced && <div className="size-8 shrink-0" aria-hidden="true" />}
			<form
				onSubmit={handleSearch}
				className="min-w-0 flex-1 md:w-200 md:flex-none"
			>
				<InputGroup className="w-full bg-primary px-2 py-6 text-primary-foreground dark:bg-primary">
					<InputGroupInput
						value={query}
						onChange={(e) => setQuery(e.target.value)}
						autoFocus={props.initial == undefined}
						className="transition ease-in placeholder:text-primary-foreground/50 focus:border-none dark:bg-transparent"
						placeholder="Search here "
					/>
					<InputGroupAddon align="inline-end">
						<SearchIcon className="text-primary-foreground" />
					</InputGroupAddon>
				</InputGroup>
			</form>
			<div className="shrink-0">
				<ModeToggle />
			</div>
		</div>
	);
}
