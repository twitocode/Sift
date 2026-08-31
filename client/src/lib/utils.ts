import { type ClassValue, clsx } from "clsx";
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

function decodePathSegment(segment: string): string {
	try {
		return decodeURIComponent(segment);
	} catch {
		return segment;
	}
}

function isAsciiSlug(segment: string): boolean {
	for (let i = 0; i < segment.length; i++) {
		if (segment.charCodeAt(i) > 127) {
			return false;
		}
	}
	return true;
}

export function getUrlWithSeperator(url: string): string[] {
	const u = new URL(url);
	const paths = u.pathname
		.split("/")
		.filter(Boolean)
		.map(decodePathSegment)
		.map((segment) => (isAsciiSlug(segment) ? toTitleCase(segment) : segment));

	return [
		`${u.protocol}//${u.hostname}`,
		...paths.flatMap((path) => [" > ", path]),
	];
}
