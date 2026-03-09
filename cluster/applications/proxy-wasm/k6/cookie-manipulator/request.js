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
        const response = http.get("http://127.0.0.1:8080/cookies", {
            headers: {
                "Cookie": "foo=modified; baz=removed",
            },
        });
        headLatency.add(response.timings.duration);
        check(response, {
            "status was 200": (r) => r.status === 200,
            "cookie was modified": (r) => r.json()["cookies"]["foo"] === "bar" && r.json()["cookies"]["baz"] === undefined,
        });
    });

    group("base", function () {
        const response = http.get("http://127.0.0.1:8081/cookies", {
            headers: {
                "Cookie": "foo=modified; baz=removed",
            },
        });
        baseLatency.add(response.timings.duration);
        check(response, {
            "status was 200": (r) => r.status === 200,
            "cookie was not modified": (r) => r.json()["cookies"]["foo"] === "modified" && r.json()["cookies"]["baz"] === "removed",
        });
    });
}
