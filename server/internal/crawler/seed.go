package crawler

// var seed = []string{"https://www.seo.com", "https://www.cnn.com", "https://www.wikipedia.org"}

var seed = []URL{
	"https://en.wikipedia.org",
	"https://www.reddit.com",
	"https://news.ycombinator.com",
	"https://www.imdb.com",
	"https://www.goodreads.com",
	"https://web.archive.org",
	"https://github.com",
	"https://www.amazon.com",
	"https://www.ebay.com",
	"https://www.pinterest.com",
	"https://www.booking.com",
	"https://www.tripadvisor.com",
	"https://www.walmart.com",
	"https://www.target.com",
	"https://www.nytimes.com",
}

var DefaultBlacklistedDomains = []string{
	// --- Search Engines & Aggregators ---
	"google.com",
	"bing.com",
	"yahoo.com",
	"duckduckgo.com",
	"baidu.com",
	"yandex.com",
	"ask.com",
	"ecosia.org",

	// --- Domain Parking & Ad Networks ---
	"sedo.com",
	"sedoparking.com",
	"parkingcrew.net",
	"bodis.com",
	"above.com",
	"hugedomains.com",
	"godaddy.com",
	"domainmarket.com",
	"namejet.com",
	"afternic.com",
	"buydomains.com",
	"dan.com",
	"squadhelp.com",
	"undev.com",
	"uniregistry.com",
	"zeroredirect1.com",
	"popads.net",
	"popcash.net",
	"adsterra.com",

	// --- Scrapers, Content Farms & Spam Aggregators ---
	"geeksforgeeks.org", // Often heavily scraped/duplicated
	"w3schools.com",
	"pinterest.com", // Deep infinite loop / doorway page issues
	"pinterest.ca",
	"pinterest.co.uk",
	"quora.com",
	"slideshare.net",

	// --- Free Subdomains & Dynamic DNS (High Abuse Potential) ---
	"000webhostapp.com",
	"ngrok.io",
	"ngrok-free.app",
	"loca.lt",
	"serveo.net",
	"pagefront.dev",
	"herokuapp.com",
	"vercel.app",  // Filter or evaluate carefully depending on use case
	"netlify.app", // Filter or evaluate carefully depending on use case
	"github.io",   // Filter or evaluate carefully depending on use case
	"wordpress.com",
	"blogspot.com",
	"weebly.com",
	"wixsite.com",
	"webnode.com",
	"jimdosite.com",
	"site123.me",
	"tumblr.com",
	"neocities.org",
	"duckdns.org",
	"no-ip.com",
	"ddns.net",
	"zapto.org",
	"hopto.org",

	// --- URL Shorteners (Usually skipped during initial crawling) ---
	"bit.ly",
	"tinyurl.com",
	"goo.gl",
	"t.co",
	"ow.ly",
	"is.gd",
	"buff.ly",
	"adf.ly",
	"bl.ink",
	"rebrand.ly",
	"cutt.ly",

	// --- Known Malware, Phishing, & Spam TLD Aggregators ---
	"bit.do",
	"shorturl.at",
	"rb.gy",

	// --- Example / Test Domains (RFC 2606) ---
	"example.com",
	"example.net",
	"example.org",
	"test.com",
	"invalid.local",
	"localhost",

	// --- Deep rabbit holes ---
	"wikipedia.org",
	"wikinews.org",
}

func GenerateBlacklistMap(blacklist []string) map[string]struct{} {
	out := make(map[string]struct{})

	for _, b := range blacklist {
		out[b] = struct{}{}
	}

	return out
}

func IsDomainBlacklisted(domain string, blacklist map[string]struct{}) bool {
	_, found := blacklist[domain]
	return found
}
