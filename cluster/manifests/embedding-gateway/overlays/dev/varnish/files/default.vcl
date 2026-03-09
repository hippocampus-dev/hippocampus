# Built-in VCL reference: https://github.com/varnishcache/varnish-cache/blob/varnish-7.6.0/bin/varnishd/builtin.vcl

vcl 4.1;

import std;

sub vcl_hash {
    # custom: include method and body hash for POST caching
    hash_data(req.method);
    if (req.http.X-Body-Hash && req.http.X-Body-Hash != "") {
        hash_data(req.http.X-Body-Hash);
    }
    # built-in: vcl_builtin_hash
    hash_data(req.url);
    if (req.http.host) {
        hash_data(req.http.host);
    } else {
        hash_data(server.ip);
    }
    return (lookup);
}

sub vcl_recv {
    # custom: healthz/metrics bypass
    if (req.method == "GET" && req.url ~ "^/(healthz|metrics)") {
        return (pass);
    }
    # custom: PURGE support (unified - BAN method not supported by Envoy)
    if (req.method == "PURGE") {
        if (req.http.X-Purge-All) {
            ban("obj.status != 0");
            return (synth(200, "All cache banned"));
        }
        if (req.http.Surrogate-Key && req.http.Surrogate-Key != "") {
            ban("obj.http.x-surrogate-key ~ " + req.http.Surrogate-Key);
            return (synth(200, "Surrogate-Key banned"));
        }
        return (purge);
    }

    # built-in: vcl_req_host
    if (req.http.host ~ "[[:upper:]]") {
        set req.http.host = req.http.host.lower();
    }
    if (!req.http.host &&
        req.esi_level == 0 &&
        req.proto == "HTTP/1.1") {
        return (synth(400));
    }
    # built-in: vcl_req_method
    if (req.method == "PRI") {
        return (synth(405));
    }
    if (req.method != "GET" &&
        req.method != "HEAD" &&
        req.method != "PUT" &&
        req.method != "POST" &&
        req.method != "TRACE" &&
        req.method != "OPTIONS" &&
        req.method != "DELETE" &&
        req.method != "PATCH") {
        set req.http.Connection = "close";
        return (synth(501));
    }

    # built-in: vcl_req_authorization
    if (req.http.Authorization) {
        return (pass);
    }
    # built-in: vcl_req_cookie
    if (req.http.Cookie) {
        return (pass);
    }

    # custom: cache POST with X-Body-Hash
    if (req.method == "POST") {
        if (!req.http.X-Body-Hash || req.http.X-Body-Hash == "") {
            return (pass);
        }
        if (!std.cache_req_body(1MB)) {
            return (pass);
        }
        set req.http.X-Original-Method = req.method;
        return (hash);
    }

    # built-in: vcl_req_method
    if (req.method != "GET" && req.method != "HEAD") {
        return (pass);
    }

    return (hash);
}

sub vcl_backend_fetch {
    set bereq.http.Host = "embedding-gateway-backend";
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
    # built-in: vcl_beresp_range
    if (beresp.status != 206 && beresp.status != 416) {
        unset beresp.http.Content-Range;
    }
    # built-in: vcl_builtin_backend_response
    if (bereq.uncacheable) {
        return (deliver);
    }

    # custom: transient errors (ttl=0s for immediate retry)
    if (beresp.status >= 429) {
        set beresp.ttl = 0s;
        set beresp.uncacheable = true;
        return (deliver);
    }

    # built-in: vcl_beresp_stale
    if (beresp.ttl <= 0s) {
        set beresp.ttl = 120s;
        set beresp.uncacheable = true;
        return (deliver);
    }
    # built-in: vcl_beresp_cookie
    if (beresp.http.Set-Cookie) {
        set beresp.ttl = 120s;
        set beresp.uncacheable = true;
        return (deliver);
    }
    # built-in: vcl_beresp_control
    if (beresp.http.Surrogate-Control ~ "(?i)no-store" ||
        (!beresp.http.Surrogate-Control &&
         beresp.http.Cache-Control ~ "(?i)(private|no-cache|no-store)")) {
        set beresp.ttl = 120s;
        set beresp.uncacheable = true;
        return (deliver);
    }
    # built-in: vcl_beresp_vary
    if (beresp.http.Vary == "*") {
        set beresp.ttl = 120s;
        set beresp.uncacheable = true;
        return (deliver);
    }

    # custom: Surrogate-Key for cache invalidation
    if (beresp.http.Surrogate-Key) {
        set beresp.http.x-surrogate-key = beresp.http.Surrogate-Key;
    }
    set beresp.ttl = 1h;
    set beresp.grace = 24h;

    return (deliver);
}

sub vcl_deliver {
    # custom: hide internal headers
    unset resp.http.Surrogate-Key;
    unset resp.http.x-surrogate-key;
}
