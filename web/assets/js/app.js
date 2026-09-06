// Shared helpers for the admin UI (fetch-based API client, formatting, toasts).

function exit() {
    localStorage.removeItem("token")
    window.location = "/index.html"
}

// api performs a JSON request against the backend, adding the admin token.
// Returns the parsed JSON body (or null for empty responses); throws an Error
// with a user-facing message on failure. A 401 redirects to the sign-in page
// unless options.redirectOnUnauthorized is false.
async function api(method, url, body = undefined, options = {}) {
    const headers = {"Content-Type": "application/json"}
    const token = localStorage.getItem("token")
    if (token) {
        headers["Authorization"] = `Bearer ${token}`
    }

    let response
    try {
        response = await fetch(url, {
            method: method,
            headers: headers,
            body: body === undefined ? undefined : JSON.stringify(body),
        })
    } catch (e) {
        console.log("ERROR", method, url, e)
        throw new Error("Cannot reach the server.")
    }

    if (response.status === 401) {
        if (options.redirectOnUnauthorized === false) {
            throw new Error("Unauthorized!")
        }
        exit()
        throw new Error("Your session has expired, please sign in again.")
    }

    let json = null
    try {
        json = await response.json()
    } catch (e) {
        // Empty body (e.g. 204) is fine.
    }

    if (!response.ok) {
        console.log("ERROR", method, url, response.status, json)
        throw new Error(json?.message || "Cannot process the request, see the error in your console.")
    }

    return json
}

function ts2date(timestamp, defaultValue = "-") {
    return timestamp ? new Date(timestamp).toLocaleDateString() : defaultValue
}

function ts2datetime(timestamp, defaultValue = "-") {
    return timestamp ? new Date(timestamp).toLocaleString() : defaultValue
}

function copyText(text) {
    if (navigator.clipboard && window.isSecureContext) {
        return navigator.clipboard.writeText(text)
    }

    return new Promise((resolve, reject) => {
        const ta = document.createElement("textarea")
        ta.value = text
        ta.setAttribute("readonly", "")
        ta.style.position = "absolute"
        ta.style.left = "-9999px"
        document.body.appendChild(ta)
        ta.select()
        ta.setSelectionRange(0, ta.value.length)
        // noinspection JSDeprecatedSymbols
        const ok = document.execCommand("copy")
        document.body.removeChild(ta)
        ok ? resolve() : reject(new Error("Copy failed."))
    })
}

// Map a node status to chip classes and a human label. When neutralUnavailable
// is true, "unavailable" renders gray instead of red — used for Pull, where the
// node simply isn't pulling yet rather than something having failed.
function statusChip(status, neutralUnavailable = false) {
    switch (status) {
        case "available":
            return {class: "chip chip-green", label: "Available"}
        case "dirty":
            return {class: "chip chip-amber", label: "Dirty"}
        case "unavailable":
            return {class: neutralUnavailable ? "chip chip-zinc" : "chip chip-red", label: "Unavailable"}
        case "disabled":
            return {class: "chip chip-zinc", label: "Disabled"}
        default:
            return {class: "chip chip-zinc", label: "Processing..."}
    }
}

document.addEventListener("alpine:init", () => {
    // Toast notifications; pages render the shared container markup.
    Alpine.store("toast", {
        items: [],
        counter: 0,
        show(message, type = "info", timeout = 3000) {
            const id = ++this.counter
            this.items.push({id: id, message: message, type: type})
            setTimeout(() => {
                this.items = this.items.filter(item => item.id !== id)
            }, timeout)
        },
        info(message) {
            this.show(message, "info")
        },
        error(message) {
            this.show(message, "error", 5000)
        },
    })
})
