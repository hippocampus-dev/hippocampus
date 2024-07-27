import http from "k6/http";
import {check, group, sleep} from "k6";
import {Trend} from "k6/metrics";

export const options = {
    stages: [
        { duration: "10s", target: 5 },
    ],
    thresholds: {
        http_req_failed: ["rate==1"],
        checks: ["rate==1"],
    },
};

const headLatency = new Trend("_head_latency");
const baseLatency = new Trend("_base_latency");

export default function () {
    group("head", function () {
        const response = http.get("http://127.0.0.1:8080/status/500");
        headLatency.add(response.timings.duration);
        check(response, { "status was 503": (r) => r.status === 503 });
        check(response, { "do fallback": (r) => r.body.toString() === "Sorry" });
        sleep(1);
    });

    group("base", function () {
        const response = http.get("http://127.0.0.1:8081/status/500");
        baseLatency.add(response.timings.duration);
        check(response, { "status was 500": (r) => r.status === 500 });
        check(response, { "do not fallback": (r) => r.body.toString() === "" });
        sleep(1);
    });
};
