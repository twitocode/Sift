import Logo from "#/components/logo.tsx";
import SearchBar from "#/components/search-bar.tsx";
import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/")({ component: Home });

function Home() {
  return (
    <div className="flex justify-center items-center flex-col gap-20 h-full w-full">
      <div className="flex flex-col items-center gap-2">
        <Logo />

      </div>
      <SearchBar />
    </div>
  );
}
