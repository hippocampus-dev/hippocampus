vcl 4.1;

import std;

sub vcl_hash {
    hash_data(req.method);
    hash_data(req.url);
    if (req.http.X-Body-Hash && req.http.X-Body-Hash != "") {
        hash_data(req.http.X-Body-Hash);
    }
    return (lookup);
}

sub vcl_recv {
    if (req.method == "GET" && req.url ~ "^/(healthz|metrics)") {
        return (pass);
    }

    if (req.method == "PURGE") {
        return (purge);
    }

    if (req.method == "BAN") {
        if (req.http.X-Purge-All) {
            ban("obj.status != 0");
            return (synth(200, "All cache banned"));
        }
        if (req.http.Surrogate-Key && req.http.Surrogate-Key != "") {
            ban("obj.http.x-surrogate-key ~ " + req.http.Surrogate-Key);
            return (synth(200, "Surrogate-Key banned"));
        }
        return (synth(400, "Missing purge header"));
    }

    if (req.method == "POST") {
        if (!req.http.X-Body-Hash || req.http.X-Body-Hash == "") {
            return (pass);
        }
        if (!std.cache_req_body(1MB)) {
            return (pass);
        }
        set req.http.X-Original-Method = req.method;
    }

    return (hash);
}

sub vcl_backend_fetch {
    set bereq.http.Host = "embedding-gateway-backend:8080";
    if (bereq.http.X-Original-Method) {
        set bereq.method = bereq.http.X-Original-Method;
    }
}

backend default {
    .host = "embedding-gateway-backend";
    .port = "8080";
    .probe = {
        .url = "/healthz";
        .interval = 1s;
        .threshold = 3;
        .window = 3;
        .timeout = 5s;
    }
}

sub vcl_backend_response {
    if (beresp.status >= 400) {
        set beresp.ttl = 0s;
        set beresp.uncacheable = true;
        return (deliver);
    }

    if (beresp.http.Surrogate-Key) {
        set beresp.http.x-surrogate-key = beresp.http.Surrogate-Key;
    }

    set beresp.ttl = 1h;
    set beresp.grace = 24h;

    return (deliver);
}

sub vcl_deliver {
    unset resp.http.Surrogate-Key;
    unset resp.http.x-surrogate-key;
}
