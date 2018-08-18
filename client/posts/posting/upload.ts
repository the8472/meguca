import lang from '../../lang'
import { load } from '../../util'
import { Post } from "../model"
import { View } from "../../base"
import { config } from "../../state"
import { postSM, postEvent } from ".";

// Precompute 00 - ff strings for conversion to hexadecimal strings
const precomputedHex = new Array(256)
for (let i = 0; i < 256; i++) {
    precomputedHex[i] = (i < 16 ? '0' : '') + i.toString(16)
}

// Uploaded file data to be embedded in thread and reply creation or file
// insertion requests
export type FileData = {
    token: string
    name: string
    spoiler?: boolean
}

interface LoadProgress {
    total: number
    loaded: number
}

// Mixin for handling file uploads
export default class UploadForm extends View<Post> {
    public spoiler: HTMLElement
    public status: HTMLElement
    public isUploading: boolean
    public input: HTMLInputElement
    private xhr: XMLHttpRequest

    constructor(model: Post, parent: HTMLElement) {
        const el = parent.querySelector(".upload-container")
        el.hidden = false
        super({ el, model })
        this.spoiler = el
            .querySelector(`span[data-id="spoiler"]`) as HTMLInputElement
        this.status = el.querySelector(".upload-status")
        this.input = el.querySelector("input[name=image]") as HTMLInputElement
    }

    // Read the file from input and send as a POST request to the server.
    // Returns image request data, if upload succeeded.
    public async uploadFile(
        file: File = this.input.files[0],
    ): Promise<FileData> {
        if (!navigator.onLine || this.isUploading) {
            return null
        }
        if (file.size > (config.maxSize << 20)) {
            this.status.textContent = "file too large"
            return null
        }

        this.isUploading = true
        this.input.style.display = "none"
        this.renderProgress({
            total: 1,
            loaded: 0,
        })

        let token: string
        // Detect, if the crypto API can be used
        if (location.protocol === "https:"
            || location.hostname === "localhost"
        ) {
            // First send a an SHA1 hash to the server, in case it already has
            // the file thumbnailed and we don't need to upload.
            const r = new FileReader()
            r.readAsArrayBuffer(file)
            const { target: { result } } = await load(r) as ArrayBufferLoadEvent;
            const hash = await crypto.subtle.digest("SHA-1", result);
            const res = await fetch("/api/upload-hash", {
                method: "POST",
                body: bufferToHex(hash),
            });
            switch (res.status) {
                case 200:
                    token = await res.text();
                    if (this.fromCloudflare(token)) {
                        return;
                    }
                    break;
                case 403:
                    const text = await res.text();
                    if (this.isCaptchaRequest(text)) {
                        postSM.feed(postEvent.captchaRequested);
                        return;
                    }
                    throw text;
                default:
                    this.isUploading = false;
                    throw await res.text();
            }
        }

        if (!token) {
            token = await this.upload(file)
            if (!token) {
                this.isUploading = false
                return null
            }
        }

        const img: FileData = {
            token,
            name: file.name,
        }
        const spoiler = (this.el
            .querySelector("input[name=spoiler]") as HTMLInputElement)
            .checked
        if (spoiler) {
            img.spoiler = true
        }
        this.isUploading = false
        return img
    }

    // Cloudflare sometimes requests a confirmation form on upload. If such a
    // form is detected, returns true and resets the upload form.
    private fromCloudflare(s: string): boolean {
        if (s.startsWith(`<!DOCTYPE html>`)) {
            window.open().document.write(s);
            this.reset();
            return true;
        }
        return false;
    }

    private isCaptchaRequest(s: string) {
        return s.indexOf("captcha required") !== -1;
    }

    // Upload file to server and return the file allocation token
    private async upload(file: File): Promise<string> {
        const formData = new FormData()
        formData.append("image", file)

        // Not using fetch, because no ProgressEvent support
        this.xhr = new XMLHttpRequest()
        this.xhr.open("POST", "/api/upload")
        this.xhr.upload.onprogress = e =>
            this.renderProgress(e)
        this.xhr.send(formData)
        await load(this.xhr)

        if (!this.isUploading) { // Cancelled
            return ""
        }
        switch (this.xhr.status) {
            case 200:
                if (this.fromCloudflare(this.xhr.responseText)) {
                    return;
                }
                break;
            case 403:
                if (this.isCaptchaRequest(this.xhr.responseText)) {
                    postSM.feed(postEvent.captchaRequested);
                    return;
                }
            default:
                this.status.textContent = this.xhr.response
                this.cancel()
                return ""
        }

        this.isUploading = false
        const text = this.xhr.responseText
        this.xhr = null
        return text
    }

    // Cancel any current uploads and reset form
    public cancel() {
        this.isUploading = false
        if (this.xhr) {
            this.xhr.abort()
            this.xhr = null
        }
        this.input.style.display = ""
    }

    // Cancel uploads and reset the view
    public reset() {
        this.cancel();
        this.status.textContent = "";
    }

    // Render client-side upload progress
    private renderProgress({ total, loaded }: LoadProgress) {
        let s: string
        if (loaded === total) {
            s = lang.ui["thumbnailing"]
        } else {
            const n = Math.floor(loaded / total * 100)
            s = `${n}% ${lang.ui["uploadProgress"]}`
        }
        this.status.textContent = s
    }
}

// Encodes an ArrayBuffer to a hex string
function bufferToHex(buf: ArrayBuffer): string {
    const b = new Uint8Array(buf),
        res = new Array(buf.byteLength)
    for (let i = 0; i < res.length; i++) {
        res[i] = precomputedHex[b[i]]
    }
    return res.join('')
}
