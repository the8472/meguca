import { makeAttrs, makeFrag, escape, on, fetchJSON } from "../util"

type OEmbedDoc = {
	title: string
	html: string
}

// Types of different embeds by provider
enum provider { Youtube, SoundCloud, Vimeo }

// Matching patterns and their respective providers
const patterns: [provider, RegExp][] = [
	[
		provider.Youtube,
		/https?:\/\/(?:[^\.]+\.)?youtube\.com\/watch\/?\?(?:.+&)?v=([^&]+)/,
	],
	[
		provider.Youtube,
		/https?:\/\/(?:[^\.]+\.)?(?:youtu\.be|youtube\.com\/embed)\/([a-zA-Z0-9_-]+)/,
	],
	[
		provider.SoundCloud,
		/https?:\/\/soundcloud.com\/.*/,
	],
	[
		provider.Vimeo,
		/https?:\/\/(?:www\.)?vimeo\.com\/.+/,
	],
]

// Map of providers to formatter functions
const formatters: { [key: number]: (s: string) => string } = {}

// Map of providers to information fetcher functions
const fetchers: { [key: number]: (el: Element) => Promise<void> } = {}

for (let p of ["Youtube", "SoundCloud", "Vimeo"]) {
	const id = (provider as any)[p] as number
	formatters[id] = formatNoEmbed(id)
	fetchers[id] = fetchNoEmbed(id)
}

// formatter for the noembed.com meta-provider
function formatNoEmbed(type: provider): (s: string) => string {
	return (href: string) => {
		const attrs = {
			rel: "noreferrer",
			href: escape(href),
			class: "embed",
			target: "_blank",
			"data-type": type.toString(),
		}
		return `<em><a ${makeAttrs(attrs)}>[${provider[type]}] ???</a></em>`
	}
}

// fetcher for the noembed.com meta-provider
function fetchNoEmbed(type: provider): (el: Element) => Promise<void> {
	return async (el: Element) => {
		const url = "https://noembed.com/embed?url=" + el.getAttribute("href"),
			[data, err] = await fetchJSON<OEmbedDoc>(url)
		if (err) {
			return console.warn(err)
		}
		el.textContent = `[${provider[type]}] ${data.title}`
		el.setAttribute("data-html", encodeURIComponent(data.html.trim()))
	}
}

// Match and parse URL against embeddable formats. If matched, returns the
// generated HTML embed string, otherwise returns empty string.
export function parseEmbeds(s: string): string {
	for (let [type, patt] of patterns) {
		if (patt.test(s)) {
			return formatters[type](s)
		}
	}
	return ""
}

// Fetch and render any metadata int the embed on mouseover
function fetchMeta(e: MouseEvent) {
	const el = e.target as Element
	if (el.hasAttribute("data-title-requested")
		|| el.classList.contains("expanded")
	) {
		return
	}
	el.setAttribute("data-title-requested", "true")
	execFetcher(el)
}

function execFetcher(el: Element): Promise<void> {
	return fetchers[parseInt(el.getAttribute("data-type"))](el)
}

// Toggle the expansion of an embed
async function toggleExpansion(e: MouseEvent) {
	// Don't trigger, when user is trying to open in a new tab
	if (e.which !== 1 || e.ctrlKey) {
		return
	}
	e.preventDefault()
	const el = e.target as Element

	if (el.classList.contains("expanded")) {
		el.classList.remove("expanded")
		const iframe = el.lastChild
		if (iframe) {
			iframe.remove()
		}
		return
	}

	// The embed was clicked before a mouseover (ex: touch screen)
	if (!el.hasAttribute("data-html")) {
		await execFetcher(el)
	}

	const html = decodeURIComponent(el.getAttribute("data-html")),
		frag = makeFrag(html)

	// Restrict embedded iframe access to the page. Improves privacy.
	for (let el of frag.querySelectorAll("iframe")) {
		el.setAttribute("referrerpolicy", "no-referrer")
		el.setAttribute(
			"sandbox",
			"allow-scripts allow-same-origin allow-popups allow-modals",
		)
	}

	el.append(frag)
	el.classList.add("expanded")
}

const threads = document.getElementById("threads")
on(threads, "mouseover", fetchMeta, {
	passive: true,
	selector: "a.embed",
})
on(threads, "click", toggleExpansion, {
	selector: "a.embed",
})

