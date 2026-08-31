import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "#/components/ui/tooltip";
import { cn, getUrlWithSeperator } from "#/lib/utils.ts";
import globe from "../assets/globe.png";
import globeDark from "../assets/globe-dark.png";
import { useTheme } from "#/components/theme-provider.tsx";

type SearchResultProps = {
	title: string;
	url: string;
	favicon: string;
	desc: string;
	ogTitle: string;
	score: number;
	titleTokens: number;
	bodyTokens: number;
};

export default function SearchResult(props: SearchResultProps) {
  const theme = useTheme()

  return (
		<Tooltip>
			<TooltipTrigger asChild>
				<div className="group min-w-0 w-full p-4 text-left transition duration-150 ease-linear hover:bg-muted/60">
					<div className="mb-2 flex min-w-0 items-center gap-3">
						<div className="flex size-10 min-h-10 min-w-10 shrink-0 items-center justify-center rounded-xl bg-muted p-2">
							<img src={props.favicon || (theme.theme == "dark" ? globe : globeDark)} alt="" />
						</div>
						<div className="flex min-w-0 flex-col justify-center">
							<p className="truncate">{props.ogTitle}</p>
							<p className="truncate text-sm text-muted-foreground">
								{getUrlWithSeperator(props.url).map((x, i) => (
									<span
										key={i}
										className={
											x !== " > " && i !== 0 ? "font-bold text-foreground" : ""
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
							"wrap-break-word text-xl font-bold text-blue-800 group-hover:underline dark:text-blue-400",
						)}
						target="_blank"
						rel="noopener noreferrer"
						href={props.url}
					>
						{props.title}
					</a>
					<p className="wrap-break-word text-sm text-muted-foreground">{props.desc}</p>
				</div>
			</TooltipTrigger>
			<TooltipContent
				side="right"
				align="center"
				sideOffset={8}
				className="flex flex-col items-start py-3 px-3"
			>
				<span className="text-lg">
					Ranking - {Math.round((props.score + Number.EPSILON) * 100) / 100}
				</span>
				{/* <span className="flex flex-col gap-1">
					<span className="">
						<span className="">Title Tokens:</span>{" "}
						<span className="text-muted-foreground">{props.titleTokens}</span>
					</span>
					<span className="">
						<span className="">Body Tokens:</span>{" "}
						<span className="text-muted-foreground">{props.bodyTokens}</span>
					</span>
				</span> */}
			</TooltipContent>
		</Tooltip>
	);
}
