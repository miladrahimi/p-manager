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

function makeErrorHandler(onMessageReceived = alert) {
    return function (jqXHR) {
        handleError(jqXHR, onMessageReceived)
    }
}

function handleError(response, onMessageReceived) {
    console.log('ERROR', response.status, response.responseText)

    const error = parseErrorMessage(response)
    if (error === "Unauthorized") {
        onMessageReceived('Your session has expired, please sign in again.')
        exit()
    } else {
        onMessageReceived(error || 'Cannot process the request, see the error in your console.')
    }
}

function parseErrorMessage(response) {
    if (response.status === 401) {
        return "Unauthorized"
    }

    if ([400, 403, 404].includes(response.status)) {
        return response?.["responseJSON"]?.["message"] || ""
    }

    return ""
}

function ts2date(timestamp, defaultValue = "-") {
    if (!timestamp) {
        return defaultValue
    }
    return (new Date(timestamp)).toLocaleDateString()
}

function ts2datetime(timestamp, defaultValue = "-") {
    if (!timestamp) {
        return defaultValue
    }

    return (new Date(timestamp)).toLocaleString()
}
