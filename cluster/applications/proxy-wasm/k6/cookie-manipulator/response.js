import http from "k6/http";
import {check, group} from "k6";
import {Trend} from "k6/metrics";

export const options = {
    scenarios: {
        test: {
            executor: "ramping-vus",
            startVUs: 1,
            stages: [
                {duration: "30s", target: 5},
            ],
            exec: "test",
        },
    },
    thresholds: {
        http_req_failed: ["rate==0"],
        checks: ["rate==1"],
    },
};

const headLatency = new Trend("_head_latency_ms");
const baseLatency = new Trend("_base_latency_ms");

export function test() {
    group("head", function () {
        const response = http.get("http://127.0.0.1:8080/cookies/set?foo=bar&qux=removed&hoge=fuga", {
            redirects: 0,
        });
        headLatency.add(response.timings.duration);
        check(response, {
            "status was 302": (r) => r.status === 302,
            "cookie was modified": (r) => r.headers["Set-Cookie"] === "foo=bar; HttpOnly; SameSite=Lax; Secure; Path=/, hoge=fuga; Path=/",
        });
    });

    group("base", function () {
        const response = http.get("http://127.0.0.1:8081/cookies/set?foo=bar&qux=removed&hoge=fuga", {
            redirects: 0,
        });
        baseLatency.add(response.timings.duration);
        check(response, {
            "status was 302": (r) => r.status === 302,
            "cookie was not modified": (r) => r.headers["Set-Cookie"] === "foo=bar; Path=/, qux=removed; Path=/, hoge=fuga; Path=/",
        });
    });
}
