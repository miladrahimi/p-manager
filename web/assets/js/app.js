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

// calendarPref returns the date calendar, preferring the reactive Alpine store
// (so views re-render on change) and falling back to localStorage.
function calendarPref() {
    try {
        if (window.Alpine && Alpine.store("prefs")) return Alpine.store("prefs").calendar
    } catch (e) {
    }
    try {
        return localStorage.getItem("calendar") || "persian"
    } catch (e) {
        return "persian"
    }
}

// dateLocale maps the calendar preference to an Intl locale. Persian uses the
// Jalali calendar (no library needed — Intl provides it).
function dateLocale() {
    return calendarPref() === "persian" ? "fa-IR-u-ca-persian" : undefined
}

function ts2date(timestamp, defaultValue = "-") {
    return timestamp ? new Date(timestamp).toLocaleDateString(dateLocale()) : defaultValue
}

function ts2datetime(timestamp, defaultValue = "-") {
    return timestamp ? new Date(timestamp).toLocaleString(dateLocale()) : defaultValue
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

// statusChip maps a node status to chip classes and a label. neutralUnavailable
// renders "unavailable" gray instead of red (used for Pull).
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

    // Admin display preferences (UI-only, persisted in this browser).
    Alpine.store("prefs", {
        calendar: (() => {
            try {
                return localStorage.getItem("calendar") || "persian"
            } catch (e) {
                return "persian"
            }
        })(),
        setCalendar(value) {
            this.calendar = value
            try {
                localStorage.setItem("calendar", value)
            } catch (e) {
            }
        },
    })
})

// Minimal front-end i18n for the public account page.
// The chosen language is kept in localStorage; Persian is the default.
const i18n = {
    languages: [{code: "fa", name: "پارسی"}, {code: "en", name: "English"}, {code: "zh", name: "中文"}],
    locales: {fa: "fa-IR", en: "en-US", zh: "zh-CN"},

    get() {
        let lang = "fa"
        try {
            lang = localStorage.getItem("lang") || lang
        } catch (e) {
        }
        return lang in this.locales ? lang : "fa"
    },

    // apply sets the document language and direction and remembers the choice.
    apply(lang) {
        document.documentElement.lang = lang
        document.documentElement.dir = lang === "fa" ? "rtl" : "ltr"
        try {
            localStorage.setItem("lang", lang)
        } catch (e) {
        }
    },

    // translate looks up key in dictionary[lang] (falling back to English) and
    // replaces {name} placeholders with params.
    translate(dictionary, lang, key, params = {}) {
        let text = dictionary[lang]?.[key] ?? dictionary.en[key] ?? key
        for (const [name, value] of Object.entries(params)) {
            text = text.replace(`{${name}}`, value)
        }
        return text
    },

    number(lang, value, digits = 2) {
        return new Intl.NumberFormat(this.locales[lang], {
            minimumFractionDigits: digits, maximumFractionDigits: digits,
        }).format(value || 0)
    },

    // date renders in the Persian (Jalali) calendar for Persian and in the
    // Gregorian calendar for every other language, regardless of browser defaults.
    date(lang, timestamp) {
        if (!timestamp) {
            return "-"
        }
        const calendar = lang === "fa" ? "persian" : "gregory"
        return new Date(timestamp).toLocaleDateString(`${this.locales[lang]}-u-ca-${calendar}`)
    },
}
