import { cn } from "#/lib/utils.ts";
import { useState } from "react";
import globe from "../assets/globe.png";

type SearchResultProps = {
  title: string;
  url: string;
  favicon: string;
  desc: string;
  ogTitle: string;
};

const toTitleCase = (str: string) =>
  str
    .split("_")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ")
    .split("-")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");

function getUrlWithSeperator(url: string): string[] {
  if (url[url.length - 1] == "/") {
    url = url.substring(0, url.length - 1);
  }

  const u = new URL(url);
  let paths = u.pathname.split("/");

  for (let i = 0; i < paths.length; i++) {
    paths[i] = toTitleCase(paths[i]);
  }
  return [u.protocol + "//" + u.hostname, ...paths.join(" > ")];
}

export default function SearchResult(props: SearchResultProps) {
  const [isHovering, setIsHovering] = useState(false);

  return (
    <div
      className="transition ease-linear hover:bg-[rgba(79,105,113,0.1)] duration-50 p-4"
      onMouseEnter={() => setIsHovering(true)}
      onMouseLeave={() => setIsHovering(false)}
    >
      <div className="flex items-center gap-3 mb-2">
        <div className="p-2 bg-[rgba(27,27,27,0.53)] size-10 min-h-10 min-w-10 rounded-xl flex items-center justify-center">
          <img src={props.favicon || globe} alt="" />
        </div>
        <div className="flex flex-col justify-center">
          <p>{props.ogTitle}</p>
          <p className="text-sm text-gray-700">
            {getUrlWithSeperator(props.url).map((x, i) => (
              <span
              key={i}
                className={x != ">" && i != 0 ? "text-black text-bold" : ""}
              >
                {x}
              </span>
            ))}
          </p>
        </div>
      </div>
      <a
        className={cn("text-blue-800 font-bold text-xl transition:underline", {
          underline: isHovering,
        })}
        target="_blank"
        rel="noopener noreferrer"
        href={props.url}
      >
        {props.title}
      </a>
      <p className="text-sm md:w-1/2 text-gray-700">{props.desc}</p>
    </div>
  );
}
