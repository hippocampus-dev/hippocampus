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
        http_req_failed: ["rate==1"],
        checks: ["rate==1"],
    },
};

const headLatency = new Trend("_head_latency_ms");
const baseLatency = new Trend("_base_latency_ms");

export function test() {
    group("head", function () {
        const response = http.get("http://127.0.0.1:8080/status/500");
        headLatency.add(response.timings.duration);
        check(response, {
            "status was 503": (r) => r.status === 503,
            "do fallback": (r) => r.body.toString() === "Sorry",
        });
    });

    group("base", function () {
        const response = http.get("http://127.0.0.1:8081/status/500");
        baseLatency.add(response.timings.duration);
        check(response, {
            "status was 500": (r) => r.status === 500,
            "do not fallback": (r) => r.body.toString() === "",
        });
    });
}
