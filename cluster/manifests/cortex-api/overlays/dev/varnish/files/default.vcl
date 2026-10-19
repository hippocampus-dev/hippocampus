# Built-in VCL reference: https://github.com/varnishcache/varnish-cache/blob/varnish-7.6.0/bin/varnishd/builtin.vcl

vcl 4.1;

import dynamic;

sub vcl_init {
    new origin = dynamic.director(port = "8080");
}

sub vcl_hash {
    # custom: the request the browser made identifies the object, not the one tls-intercept-proxy rewrote
    hash_data(req.url);
    if (req.http.X-Original-Scheme) {
        hash_data(req.http.X-Original-Scheme);
    }
    if (req.http.X-Original-Host) {
        hash_data(req.http.X-Original-Host);
    } else if (req.http.host) {
        hash_data(req.http.host);
    } else {
        hash_data(server.ip);
    }
    return (lookup);
}

sub vcl_recv {
    set req.backend_hint = origin.backend("cortex-api-tls-intercept-proxy");

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
        return (pipe);
    }
    if (req.method != "GET" && req.method != "HEAD") {
        return (pass);
    }
    # built-in: vcl_req_authorization
    if (req.http.Authorization) {
        return (pass);
    }
    # built-in: vcl_req_cookie
    if (req.http.Cookie) {
        return (pass);
    }

    return (hash);
}

backend default {
    .host = "127.0.0.1";
    .port = "80";
}

sub vcl_backend_fetch {
    set bereq.http.Host = "cortex-api-tls-intercept-proxy";
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

    # custom: default_ttl would otherwise store responses an arbitrary site never declared cacheable
    if (!beresp.http.Cache-Control && !beresp.http.Expires) {
        set beresp.ttl = 120s;
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

    return (deliver);
}

sub vcl_deliver {
    # custom: obj.uncacheable is true for hit-for-pass and hit-for-miss too, not only pass (https://github.com/varnishcache/varnish-cache/blob/varnish-7.6.0/doc/sphinx/reference/vcl_var.rst)
    if (obj.uncacheable) {
        set resp.http.X-Cache = "PASS";
    } else if (obj.hits > 0) {
        set resp.http.X-Cache = "HIT";
    } else {
        set resp.http.X-Cache = "MISS";
    }
}
