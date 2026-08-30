import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export const toTitleCase = (str: string) =>
  str
    .split("_")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ")
    .split("-")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");

export function getUrlWithSeperator(url: string): string[] {
  const u = new URL(url);
  const paths = u.pathname
    .split("/")
    .filter(Boolean)
    .map(toTitleCase);

  return [
    `${u.protocol}//${u.hostname}`,
    ...paths.flatMap((path) => [" > ", path]),
  ];
}
