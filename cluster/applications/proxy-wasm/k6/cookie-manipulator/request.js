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
        const response = http.get("http://127.0.0.1:8080/cookies", {
            headers: {
                "Cookie": "foo=modified; baz=removed",
            },
        });
        headLatency.add(response.timings.duration);
        check(response, { "status was 200": (r) => r.status === 200 });
        check(response, { "cookie was modified": (r) => r.json()["cookies"]["foo"] === "bar" && r.json()["cookies"]["baz"] === undefined });
        sleep(1);
    });

    group("base", function () {
        const response = http.get("http://127.0.0.1:8081/cookies", {
            headers: {
                "Cookie": "foo=modified; baz=removed",
            },
        });
        baseLatency.add(response.timings.duration);
        check(response, { "status was 200": (r) => r.status === 200 });
        check(response, { "cookie was not modified": (r) => r.json()["cookies"]["foo"] === "modified" && r.json()["cookies"]["baz"] === "removed" });
        sleep(1);
    });
};
