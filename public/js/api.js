// Gophish API wrapper adapted for JWT auth (Vercel version)

function getToken() {
    return localStorage.getItem('gophish_token') || ''
}

function authHeaders() {
    return {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer ' + getToken()
    }
}

function query(path, method, data, raw) {
    var opts = {
        method: method || 'GET',
        headers: authHeaders(),
        credentials: 'include'
    }
    if (data && method !== 'GET') {
        opts.body = JSON.stringify(data)
    }
    var p = fetch('/api' + path, opts).then(function(r) {
        if (r.status === 401) {
            localStorage.removeItem('gophish_token')
            window.location = '/login'
        }
        return r.json()
    })
    if (raw) return p
    // jQuery-style .success/.error chaining
    var handlers = { success: null, error: null }
    p.then(function(data) {
        if (data && data.message && handlers._isError) {
            handlers.error && handlers.error({ responseJSON: data })
        } else {
            handlers.success && handlers.success(data)
        }
    }).catch(function(e) {
        handlers.error && handlers.error({ responseJSON: { message: e.message } })
    })
    var obj = {
        success: function(fn) { handlers.success = fn; return obj },
        error:   function(fn) { handlers.error   = fn; return obj },
        done:    function(fn) { p.then(fn); return obj },
        fail:    function(fn) { p.catch(fn); return obj },
        always:  function(fn) { p.finally(fn); return obj }
    }
    return obj
}

var api = {
    campaigns: {
        get:     function() { return query('/campaigns/', 'GET') },
        post:    function(c) { return query('/campaigns/', 'POST', c) },
        summary: function() { return query('/campaigns/summary', 'GET') }
    },
    campaignId: {
        get:      function(id) { return query('/campaigns/' + id, 'GET') },
        delete:   function(id) { return query('/campaigns/' + id, 'DELETE') },
        results:  function(id) { return query('/campaigns/' + id + '/results', 'GET') },
        summary:  function(id) { return query('/campaigns/' + id + '/summary', 'GET') },
        complete: function(id) { return query('/campaigns/' + id + '/complete', 'GET') }
    },
    groups: {
        get:     function() { return query('/groups/', 'GET') },
        post:    function(g) { return query('/groups/', 'POST', g) },
        summary: function() { return query('/groups/summary', 'GET') }
    },
    groupId: {
        get:    function(id) { return query('/groups/' + id, 'GET') },
        put:    function(id, g) { return query('/groups/' + id, 'PUT', g) },
        delete: function(id) { return query('/groups/' + id, 'DELETE') },
        summary: function(id) { return query('/groups/' + id + '/summary', 'GET') }
    },
    templates: {
        get:  function() { return query('/templates/', 'GET') },
        post: function(t) { return query('/templates/', 'POST', t) }
    },
    templateId: {
        get:    function(id) { return query('/templates/' + id, 'GET') },
        put:    function(id, t) { return query('/templates/' + id, 'PUT', t) },
        delete: function(id) { return query('/templates/' + id, 'DELETE') }
    },
    pages: {
        get:  function() { return query('/pages/', 'GET') },
        post: function(p) { return query('/pages/', 'POST', p) }
    },
    pageId: {
        get:    function(id) { return query('/pages/' + id, 'GET') },
        put:    function(id, p) { return query('/pages/' + id, 'PUT', p) },
        delete: function(id) { return query('/pages/' + id, 'DELETE') }
    },
    SMTP: {
        get:  function() { return query('/smtp/', 'GET') },
        post: function(s) { return query('/smtp/', 'POST', s) }
    },
    SMTPId: {
        get:    function(id) { return query('/smtp/' + id, 'GET') },
        put:    function(id, s) { return query('/smtp/' + id, 'PUT', s) },
        delete: function(id) { return query('/smtp/' + id, 'DELETE') }
    },
    send_test_email: function(req) { return query('/util/send_test_email', 'POST', req) },
    import: {
        site: function(req) { return query('/import/site', 'POST', req) },
        email: function(req) { return query('/import/email', 'POST', req) },
        group: function(req) { return query('/import/group', 'POST', req) }
    }
}
