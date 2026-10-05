vcl 4.1;

backend default {
    .host = "127.0.0.1";
    .port = "8080";
}

sub vcl_recv {
    return (pass);
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
