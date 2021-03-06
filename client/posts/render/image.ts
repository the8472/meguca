import { config } from '../../state'
import options from '../../options'
import { escape, setAttrs, pad, importTemplate, firstChild } from '../../util'
import { ImageData, fileTypes } from '../../common'
import lang from '../../lang'

// Specs for handling image search link clicks
type ImageSearchSpec = {
    type: ISType
    url: string
}

// Types of data requested by the search provider
const enum ISType { src, MD5, SHA1 }

const ISSpecs: ImageSearchSpec[] = [
    {
        type: ISType.src,
        url: "https://www.google.com/searchbyimage?image_url=",
    },
    {
        type: ISType.src,
        url: "http://iqdb.org/?url=",
    },
    {
        type: ISType.src,
        url: "http://saucenao.com/search.php?db=999&url=",
    },
    {
        type: ISType.src,
        url: "https://whatanime.ga/?url=",
    },
    {
        type: ISType.MD5,
        url: "https://desuarchive.org/_/search/image/",
    },
    {
        type: ISType.SHA1,
        url: "http://exhentai.org/?fs_similar=1&fs_exp=1&f_shash=",
    },
]

// Render a thumbnail of an image, according to configuration settings
export function renderImage(post: Element, data: ImageData, reveal: boolean) {
    // Need to find direct descendant, otherwise inlined posts might match
    const cont = post.querySelector(".post-container")
    let el = firstChild(cont, ch =>
        ch.tagName === "FIGURE")
    if (!el) {
        el = importTemplate("figure").firstChild as HTMLElement
        cont.prepend(el)
    }

    const showThumb = (!options.hideThumbs && !options.workModeToggle) || reveal
    el.hidden = !showThumb
    if (showThumb) {
        (el.firstElementChild as HTMLElement).hidden = false
    }
    if (showThumb) {
        renderThumbnail(el.lastElementChild, data)
    }
    renderFigcaption(post, data, reveal)
}

// Render the information caption above the image
function renderFigcaption(post: Element, data: ImageData, reveal: boolean) {
    let el = firstChild(post, ch =>
        ch.tagName === "FIGCAPTION")
    if (!el) {
        el = importTemplate("figcaption").firstChild as HTMLElement
        post.querySelector("header").after(el)
    }

    const list: string[] = []
    if (data.audio) {
        list.push('\u266B')
    }
    if (data.length) {
        list.push(readableLength(data.length))
    }
    if (data.apng) {
        list.push('APNG')
    }
    list.push(readableFilesize(data.size), `${data.dims[0]}x${data.dims[1]}`)

    const [hToggle, , info, link] = Array.from(el.children) as HTMLElement[]
    if (!options.hideThumbs && !options.workModeToggle) {
        hToggle.hidden = true
    } else {
        hToggle.hidden = false
        hToggle.textContent = lang.posts[reveal ? 'hide' : 'show']
    }
    info.textContent = `(${commaList(list)})`
    imageLink(link, data)
    renderImageSearch(el.querySelector(".image-search-container"), data)
    el.hidden = false
}

// Makes a ', ' separated list
function commaList(items: string[]): string {
    let html = ''
    for (let item of items) {
        if (html) {
            html += ', '
        }
        html += item
    }
    return html
}

// Assign URLs to image search links
function renderImageSearch(cont: HTMLElement, img: ImageData) {
    const ch = cont.children
    for (let i = 0; i < ch.length; i++) {
        const { type, url } = ISSpecs[i]
        let arg: string
        switch (type) {
            case ISType.src:
                arg = resolveFuzzyIS(img)
                break
            case ISType.MD5:
                arg = img.MD5
                break
            case ISType.SHA1:
                arg = img.SHA1
                break
        }
        ch[i].setAttribute("href", url + arg)
    }
}

// Resolve URL of image search, that require to download the image file
function resolveFuzzyIS({ fileType, thumbType, SHA1, size }: ImageData): string {
    let root: string,
        type: fileTypes
    switch (fileType) {
        case fileTypes.jpg:
        case fileTypes.gif:
        case fileTypes.png:
            if (size > 8 << 20) {
                root = "thumb"
                type = thumbType
            } else {
                root = "src"
                type = fileType
            }
            break
        default:
            root = "thumb"
            type = thumbType
    }
    const s = `/images/${root}/${SHA1}.${fileTypes[type]}`
    return encodeURI(location.origin + s)
}

// Render video/audio length in human readable form
function readableLength(len: number): string {
    if (len < 60) {
        return `0:${pad(len)}`
    }
    const min = Math.floor(len / 60),
        sec = len - min * 60
    return `${pad(min)}:${pad(sec)}`
}

// Renders a human readable file size string
function readableFilesize(size: number): string {
    if (size < (1 << 10)) {
        return size + ' B'
    }
    if (size < (1 << 20)) {
        return Math.round(size / (1 << 10)) + ' KB'
    }
    const text = Math.round(size / (1 << 20) * 10).toString()
    return `${text.slice(0, -1)}.${text.slice(-1)} MB`
}

function imageRoot(): string {
    return config.imageRootOverride || "/images"
}

// Get the thumbnail path of an image, accounting for not thumbnail of specific
// type being present
export function thumbPath(SHA1: string, thumbType: fileTypes): string {
    return `${imageRoot()}/thumb/${SHA1}.${fileTypes[thumbType]}`
}

// Resolve the path to the source file of an upload
export function sourcePath(SHA1: string, fileType: fileTypes): string {
    return `${imageRoot()}/src/${SHA1}.${fileTypes[fileType]}`
}

// Resolve the path to the source file of an upload relative to the site root
function relativeSourcePath(SHA1: string, fileType: fileTypes): string {
    return `/images/src/${SHA1}.${fileTypes[fileType]}`
}

// Render a name + download link of an image
function imageLink(el: Element, data: ImageData) {
    let { name } = data
    const { SHA1, fileType } = data
    name = `${escape(name)}.${fileTypes[fileType]}`

    setAttrs(el, {
        href: relativeSourcePath(SHA1, fileType),
        download: name,
    })
    el.innerHTML = name
}

// Render the actual thumbnail image
function renderThumbnail(el: Element, data: ImageData) {
    const src = sourcePath(data.SHA1, data.fileType)
    let thumb: string,
        [, , thumbWidth, thumbHeight] = data.dims

    if (data.spoiler && options.spoilers) {
        // Spoilered and spoilers enabled
        thumb = '/assets/spoil/default.jpg'
        thumbHeight = thumbWidth = 150
    } else if (data.fileType === fileTypes.gif && options.autogif) {
        // Animated GIF thumbnails
        thumb = src
    } else {
        thumb = thumbPath(data.SHA1, data.thumbType)
    }

    // Downscale thumbnail for higher DPI, unless specified not to
    if (!data.large && (thumbWidth > 125 || thumbHeight > 125)) {
        thumbWidth *= 0.8333
        thumbHeight *= 0.8333
    }

    el.setAttribute("href", src)
    setAttrs(el.firstElementChild, {
        src: thumb,
        width: thumbWidth.toString(),
        height: thumbHeight.toString(),
        class: "", // Remove any existing classes
    })
}
