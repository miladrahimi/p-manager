$.ajaxSetup({
    headers: {
        'Content-Type': "application/json",
        'Authorization': `Bearer ${localStorage.getItem("token")}`,
    }
})

function exit() {
    localStorage.removeItem("token")
    window.location = "index.html"
}

$('#exit').click(exit)

function handleAuthError(response) {
    if (response.status === 401) {
        exit()
    }
}

function ts2string(timestamp) {
    if (!timestamp) {
        return "-"
    }
    let d = (new Date(timestamp)).toLocaleDateString('fa-IR')
    return d.replace(/[\u0660-\u0669\u06f0-\u06f9]/g, function (c) {
        return c.charCodeAt(0) & 0xf
    })
}
