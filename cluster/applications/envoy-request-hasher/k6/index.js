import http from "k6/http";
import {check, group} from "k6";
import {Trend} from "k6/metrics";
import {sha256} from "k6/crypto";

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

function findHeader(headers, name) {
    const lower = name.toLowerCase();
    for (const key in headers) {
        if (key.toLowerCase() === lower) {
            return headers[key];
        }
    }
    return undefined;
}

const body = '{"message":"hello"}';
const path = "/post";
const url = "/post?foo=bar";

const expectedBodyHash = sha256(body, "hex");
const expectedPathHash = sha256(path, "hex");
const expectedUrlHash = sha256(url, "hex");

export function test() {
    group("head", function () {
        const response = http.post("http://127.0.0.1:8080" + url, body, {
            headers: {
                "Content-Type": "application/json",
            },
        });
        headLatency.add(response.timings.duration);

        const json = response.json();
        const headers = json.headers;

        check(response, {
            "status was 200": (r) => r.status === 200,
        });
        check(headers, {
            "x-body-hash present": (h) => findHeader(h, "X-Body-Hash") !== undefined,
            "x-body-hash correct": (h) => findHeader(h, "X-Body-Hash") === expectedBodyHash,
            "x-path-hash present": (h) => findHeader(h, "X-Path-Hash") !== undefined,
            "x-path-hash correct": (h) => findHeader(h, "X-Path-Hash") === expectedPathHash,
            "x-url-hash present": (h) => findHeader(h, "X-Url-Hash") !== undefined,
            "x-url-hash correct": (h) => findHeader(h, "X-Url-Hash") === expectedUrlHash,
        });
    });

    group("base", function () {
        const response = http.post("http://127.0.0.1:8081" + url, body, {
            headers: {
                "Content-Type": "application/json",
            },
        });
        baseLatency.add(response.timings.duration);

        const json = response.json();
        const headers = json.headers;

        check(response, {
            "status was 200": (r) => r.status === 200,
        });
        check(headers, {
            "x-body-hash absent": (h) => findHeader(h, "X-Body-Hash") === undefined,
            "x-path-hash absent": (h) => findHeader(h, "X-Path-Hash") === undefined,
            "x-url-hash absent": (h) => findHeader(h, "X-Url-Hash") === undefined,
        });
    });
}
