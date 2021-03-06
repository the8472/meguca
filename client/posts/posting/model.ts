import { message, send, handlers } from "../../connection"
import { Post } from "../model"
import { ImageData, PostData } from "../../common"
import FormView from "./view"
import { posts, storeMine, page } from "../../state"
import { postSM, postEvent, postState } from "."
import { extend } from "../../util"
import { SpliceResponse } from "../../client"
import { FileData } from "./upload"
import { newAllocRequest } from "./identity"

// Form Model of an OP post
export default class FormModel extends Post {
	public sentAllocRequest: boolean
	public isAllocated: boolean
	public nonLive: boolean // Disable live post updates

	// Text that is not submitted yet to defer post allocation
	public bufferedText: string

	public inputBody: string
	public view: FormView
	private lasLinked: number // ID of last linked post

	// Pass and ID, if you wish to hijack an existing model. To create a new
	// model pass zero.
	constructor(id: number) {
		if (id !== 0) {
			storeMine(id)

			const oldModel = posts.get(id),
				oldView = oldModel.view
			oldView.unbind()

			// Copy the parent model's state and data
			const attrs = {} as PostData
			for (let key in oldModel) {
				if (typeof oldModel[key] !== "function") {
					attrs[key] = oldModel[key]
				}
			}
			super(attrs)
			this.sentAllocRequest = this.isAllocated = true

			// Replace old model and view pair with the postForm pair
			posts.add(this)
			const view = new FormView(this, true)
			oldView.el.replaceWith(view.el)

			postSM.feed(postEvent.hijack, { view, model: this })
		} else {
			super({
				id: 0,
				op: page.thread,
				editing: true,
				deleted: false,
				banned: false,
				time: Date.now(),
				body: "",
				state: {
					spoiler: false,
					quote: false,
					lastLineEmpty: false,
					code: false,
					haveSyncwatch: false,
					iDice: 0,
				},
			})
		}

		// Initialize state
		this.inputBody = ""
	}

	// Append a character to the model's body and reparse the line, if it's a
	// newline
	public append(code: number) {
		if (!this.editing) {
			return
		}
		this.body += String.fromCodePoint(code)
	}

	// Remove the last character from the model's body
	public backspace() {
		if (!this.editing) {
			return
		}
		this.body = this.body.slice(0, -1)
	}

	// Splice the last line of the body
	public splice(msg: SpliceResponse) {
		if (!this.editing) {
			return
		}
		this.body = this.spliceText(this.body, msg)
	}

	// Compare new value to old and generate appropriate commands
	public parseInput(val: string): void {
		// Handle live update toggling
		if (this.nonLive) {
			this.bufferedText = val
			return
		}

		// Remove any buffered quote, as we are committing now
		this.bufferedText = ""

		const old = this.inputBody

		// Rendering hack shenanigans - ignore
		if (old === val) {
			return
		}

		const lenDiff = val.length - old.length,
			exceeding = old.length + lenDiff - 2000

		// If exceeding max body length, shorten the value, trim input and try
		// again
		if (exceeding > 0) {
			this.view.trimInput(exceeding)
			return this.parseInput(val.slice(0, -exceeding))
		}

		// Remove any lines past 30
		const lines = val.split("\n")
		if (lines.length - 1 > 100) {
			const trimmed = lines.slice(0, 100).join("\n")
			this.view.trimInput(val.length - trimmed.length)
			return this.parseInput(trimmed)
		}

		if (!this.sentAllocRequest) {
			return this.requestAlloc(val, null)
		}

		if (lenDiff === 1 && val.slice(0, -1) === old) {
			return this.commitChar(val.slice(-1))
		}
		if (lenDiff === -1 && old.slice(0, -1) === val) {
			return this.commitBackspace()
		}

		return this.commitSplice(val)
	}

	// Commit a character appendage to the end of the line to the server
	private commitChar(char: string) {
		this.inputBody += char
		this.send(message.append, char.codePointAt(0))
	}

	// Optionally buffer all data, if currently disconnected
	private send(type: message, msg: any) {
		if (postSM.state !== postState.halted) {
			send(type, msg)
		}
	}

	// Send a message about removing the last character of the line to the
	// server
	private commitBackspace() {
		this.inputBody = this.inputBody.slice(0, -1)
		this.send(message.backspace, null)
	}

	// Commit any other input change that is not an append or backspace
	private commitSplice(v: string) {
		// Convert to arrays of chars to deal with multibyte unicode chars
		const old = [...this.inputBody],
			val = [...v]
		let start: number

		// Find first differing character
		for (let i = 0; i < old.length; i++) {
			if (old[i] !== val[i]) {
				start = i
				break
			}
		}

		// New string is appended to the end
		if (start === undefined) {
			start = old.length
		}

		// Right now we simply resend the entire corrected string, including the
		// common part, because I can't figure out a diff algorithm that covers
		// all cases. The backend technically supports the latter.
		const end = val.slice(start).join("")
		this.send(message.splice, {
			start,
			len: -1,
			text: end,
		})
		this.inputBody = old.slice(0, start).join("") + end
	}

	// Close the form and revert to regular post
	public commitClose() {
		// It is possible to have never committed anything, if all you have in
		// the body is one quote and an image allocated.
		if (this.bufferedText) {
			this.nonLive = false
			this.parseInput(this.bufferedText)
		}

		this.body = this.inputBody
		this.abandon()
		this.send(message.closePost, null)
	}

	// Turn post form into a regular post, because it has expired after a
	// period of posting ability loss
	public abandon() {
		this.view.cleanUp()
		this.closePost()
	}

	// Add a link to the target post in the input
	public addReference(id: number, sel: string) {
		let s = ""
		const old = this.bufferedText || this.inputBody,
			newLine = !old || old.endsWith("\n")

		// Don't duplicate links, if quoting same post multiple times in
		// succession
		if (id !== this.lasLinked) {
			if (!newLine && old[old.length - 1] !== " ") {
				s += " "
			}
			s += `>>${id} `
		}
		this.lasLinked = id

		if (!sel) {
			// If starting from a new line, insert newline after post
			if (newLine) {
				s += "\n"
			}
		} else {
			s += "\n"
			for (let line of sel.split("\n")) {
				s += ">" + line + "\n"
			}
		}

		// Don't commit a quote, if it is the first input in a post
		let commit = true
		if (!this.sentAllocRequest && !this.bufferedText) {
			commit = false
		}
		this.view.replaceText(old + s, commit)

		// Makes sure the quote is committed later, if it is the first input in
		// the post
		if (!commit) {
			this.bufferedText = s
		}
	}

	// Request allocation of a draft post to the server
	private requestAlloc(body: string | null, image: FileData | null) {
		this.view.removeLiveToggle()
		this.view.setEditing(true)
		this.nonLive = false
		this.sentAllocRequest = true
		const req = newAllocRequest()

		if (body) {
			req["body"] = body
			this.body = body
			this.inputBody = body
		}
		if (image) {
			req["image"] = image
		}

		send(message.insertPost, req)
		handlers[message.postID] = (id: number) => {
			this.setID(id)
			delete handlers[message.postID]
		}
	}

	// Set post ID and add to the post collection
	private setID(id: number) {
		this.id = id
		postSM.feed(postEvent.alloc)
		posts.add(this)
	}

	// Handle draft post allocation
	public onAllocation(data: PostData) {
		// May sometimes be called multiple times, because of reconnects
		if (this.isAllocated) {
			return
		}

		this.isAllocated = true
		extend(this, data)
		this.view.renderAlloc()
		storeMine(data.id)
		if (data.image) {
			this.insertImage(this.image)
		}
	}

	// Upload the file and request its allocation
	public async uploadFile(file?: File) {
		// Already have image
		if (this.image) {
			return
		}

		const data = await this.view.upload.uploadFile(file)
		// Upload failed, canceled, image added while thumbnailing or post
		// closed
		if (!data || this.image || !this.editing) {
			return
		}

		if (!this.sentAllocRequest) {
			this.requestAlloc(null, data)
		} else {
			send(message.insertImage, data)
		}
	}

	// Insert the uploaded image into the model
	public insertImage(img: ImageData) {
		this.image = img
		this.view.insertImage()
	}

	// Spoiler an already allocated image
	public commitSpoiler() {
		this.send(message.spoiler, null)
	}
}
