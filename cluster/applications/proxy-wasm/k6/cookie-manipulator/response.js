import http from "k6/http";
import {check, group, sleep} from "k6";
import {Trend} from "k6/metrics";

export const options = {
    stages: [
        { duration: "10s", target: 5 },
    ],
    thresholds: {
        http_req_failed: ["rate==0"],
        checks: ["rate==1"],
    },
};

const headLatency = new Trend("_head_latency");
const baseLatency = new Trend("_base_latency");

export default function () {
    group("head", function () {
        const response = http.get("http://127.0.0.1:8080/cookies/set?foo=bar&qux=removed&hoge=fuga", {
            redirects: 0,
        });
        headLatency.add(response.timings.duration);
        check(response, { "status was 302": (r) => r.status === 302 });
        check(response, { "cookie was modified": (r) => r.headers["Set-Cookie"] === "foo=bar; HttpOnly; SameSite=Lax; Secure; Path=/, hoge=fuga; Path=/" });
        sleep(1);
    });

    group("base", function () {
        const response = http.get("http://127.0.0.1:8081/cookies/set?foo=bar&qux=removed&hoge=fuga", {
            redirects: 0,
        });
        baseLatency.add(response.timings.duration);
        check(response, { "status was 302": (r) => r.status === 302 });
        check(response, { "cookie was not modified": (r) => r.headers["Set-Cookie"] === "foo=bar; Path=/, qux=removed; Path=/, hoge=fuga; Path=/" });
        sleep(1);
    });
};
