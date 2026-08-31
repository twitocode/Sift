import { useNavigate } from "@tanstack/react-router";
import { SearchIcon } from "lucide-react";
import { useState } from "react";
import { ModeToggle } from "#/components/mode-toggle.tsx";
import {
	InputGroup,
	InputGroupAddon,
	InputGroupInput,
} from "#/components/ui/input-group.tsx";

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

	const form = (
		<form onSubmit={handleSearch} className="min-w-0 w-full flex-1">
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
	);

	if (!props.balanced) {
		return form;
	}

	return (
		<div className="flex w-full min-w-0 max-w-xl items-center gap-2">
			<div className="size-8 shrink-0" aria-hidden="true" />
			{form}
			<div className="shrink-0">
				<ModeToggle />
			</div>
		</div>
	);
}
