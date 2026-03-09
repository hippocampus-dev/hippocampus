import http from "k6/http";
import {check, group} from "k6";
import {Gauge, Trend} from "k6/metrics";

export const options = {
    scenarios: {
        test: {
            executor: "ramping-vus",
            startVUs: 10,
            stages: [
                {duration: "30s", target: 10},
            ],
            exec: "test",
        },
        monitor: {
            executor: "constant-arrival-rate",
            duration: "30s",
            rate: 1,
            timeUnit: "1s",
            preAllocatedVUs: 1,
            exec: "monitor",
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
        const response = http.get("http://127.0.0.1:18888");
        headLatency.add(response.timings.duration);
        check(response, {
            "status was 200": (r) => r.status === 200,
        });
    });

    group("base", function () {
        const response = http.get("http://127.0.0.1:8888");
        baseLatency.add(response.timings.duration);
        check(response, {
            "status was 200": (r) => r.status === 200,
        });
    });
}

function parseValue(s) {
    const parts = s.split(" ");
    return parseInt(parts[parts.length - 1]);
}

const connections = new Gauge("__connections");
const idleConnections = new Gauge("__idle_connections");

export function monitor() {
    const response = http.get("http://127.0.0.1:8080/metrics");
    const metrics = response.body;
    if (typeof metrics === "string") {
        metrics.split("\n").forEach((line) => {
            if (line.startsWith("tcp_connections")) {
                connections.add(parseValue(line));
            } else if (line.startsWith("tcp_idle_connections")) {
                idleConnections.add(parseValue(line));
            }
        });
    }
}