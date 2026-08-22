import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from "#/components/ui/input-group.tsx";
import { useNavigate } from "@tanstack/react-router";
import { SearchIcon } from "lucide-react";
import { useState } from "react";

type SearchBarProps = {
  initial?: string
}
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
    <form onSubmit={handleSearch}>
      <InputGroup className="w-200 py-6 px-2 bg-primary text-primary-foreground">
        <InputGroupInput
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          autoFocus
          className="transition ease-in focus:border-none"
          placeholder="Search here "
        />
        <InputGroupAddon align="inline-end">
          <SearchIcon className="text-white" />
        </InputGroupAddon>
      </InputGroup>
    </form>
  );
}
