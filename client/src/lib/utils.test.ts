import { describe, expect, test } from "bun:test";
import { getUrlWithSeperator } from "./utils.ts";

describe("getUrlWithSeperator", () => {
	test("decodes percent-encoded path segments", () => {
		const url =
			"https://hy.wikipedia.org/wiki/%D5%95%D5%BD%D5%B4%D5%A1%D5%B6%D5%B5%D5%A1%D5%B6_%D5%AF%D5%A1%D5%B5%D5%BD%D6%80%D5%B8%D6%82%D5%A9%D5%B5%D5%B8%D6%82%D5%B6";

		expect(getUrlWithSeperator(url)).toEqual([
			"https://hy.wikipedia.org",
			" > ",
			"Wiki",
			" > ",
			"Օսմանյան_կայսրություն",
		]);
	});

	test("title-cases ASCII slugs but leaves non-latin segments unchanged", () => {
		expect(getUrlWithSeperator("https://example.com/my-page-title")).toEqual([
			"https://example.com",
			" > ",
			"My Page Title",
		]);

		expect(
			getUrlWithSeperator(
				"https://hy.wikipedia.org/wiki/Օսմանյան_կայսրություն",
			),
		).toEqual([
			"https://hy.wikipedia.org",
			" > ",
			"Wiki",
			" > ",
			"Օսմանյան_կայսրություն",
		]);
	});
});
