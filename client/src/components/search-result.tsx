import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "#/components/ui/tooltip";
import { cn, getUrlWithSeperator } from "#/lib/utils.ts";
import globe from "../assets/globe.png";

type SearchResultProps = {
  title: string;
  url: string;
  favicon: string;
  desc: string;
  ogTitle: string;
  score: number;
};

export default function SearchResult(props: SearchResultProps) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className="group w-full p-4 text-left transition duration-150 ease-linear hover:bg-[rgba(79,105,113,0.1)]">
          <div className="flex items-center gap-3 mb-2">
            <div className="p-2 bg-[rgba(27,27,27,0.53)] size-10 min-h-10 min-w-10 rounded-xl flex items-center justify-center">
              <img src={props.favicon || globe} alt="" />
            </div>
            <div className="flex flex-col justify-center">
              <p>{props.ogTitle}</p>
              <p className="text-sm text-gray-600">
                {getUrlWithSeperator(props.url).map((x, i) => (
                  <span
                    key={i}
                    className={
                      x !== " > " && i !== 0 ? "font-bold text-gray-700" : ""
                    }
                  >
                    {x}
                  </span>
                ))}
              </p>
            </div>
          </div>
          <a
            className={cn(
              "text-xl font-bold text-blue-800 group-hover:underline",
            )}
            target="_blank"
            rel="noopener noreferrer"
            href={props.url}
          >
            {props.title}
          </a>
          <p className="text-sm md:w-1/2 text-gray-700">{props.desc}</p>
        </div>
      </TooltipTrigger>
      <TooltipContent>
        <span>{Math.round((props.score + Number.EPSILON) * 100) / 100}</span>
      </TooltipContent>
    </Tooltip>
  );
}
