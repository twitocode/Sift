package indexer

// Common DNS TLDs kept attached to hostnames (github.com, docs.github.io).
var tlds = map[string]struct{}{
	"com": {}, "org": {}, "net": {}, "edu": {}, "gov": {}, "mil": {}, "int": {},
	"io": {}, "co": {}, "uk": {}, "us": {}, "ca": {}, "au": {}, "de": {}, "fr": {},
	"jp": {}, "cn": {}, "in": {}, "br": {}, "ru": {}, "nl": {}, "se": {}, "no": {},
	"es": {}, "it": {}, "ch": {}, "pl": {}, "be": {}, "at": {}, "dk": {}, "fi": {},
	"ie": {}, "nz": {}, "za": {}, "mx": {}, "kr": {}, "sg": {}, "hk": {}, "tw": {},
	"il": {}, "tr": {}, "pt": {}, "cz": {}, "gr": {}, "hu": {}, "ro": {}, "ua": {},
	"ai": {}, "app": {}, "dev": {}, "cloud": {}, "tech": {}, "online": {}, "site": {},
	"xyz": {}, "info": {}, "biz": {}, "name": {}, "pro": {}, "me": {}, "tv": {},
	"cc": {}, "ws": {}, "gg": {}, "to": {}, "so": {}, "sh": {}, "fm": {}, "ly": {},
	"page": {}, "blog": {}, "shop": {}, "store": {}, "news": {}, "live": {},
	"space": {}, "website": {}, "email": {}, "link": {}, "zip": {}, "mov": {},
}
