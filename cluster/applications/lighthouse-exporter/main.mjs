import {parseArgs} from "node:util";
import {URL} from "node:url";
import puppeteer from "puppeteer";
import lighthouse, {desktopConfig} from "lighthouse";
import {MeterProvider} from "@opentelemetry/sdk-metrics";
import {PrometheusExporter} from "@opentelemetry/exporter-prometheus";
import {HostMetrics} from "@opentelemetry/host-metrics";

const {
    positionals: urls,
    values: {
        "form-factors": formFactors,
        audits,
        "force-tracing": forceTracing,
        interval,
        port,
    }
} = parseArgs({
    options: {
        "form-factors": {
            type: "string",
            multiple: true,
            default: ["desktop", "mobile"],
        },
        // https://github.com/GoogleChrome/lighthouse/tree/HEAD/core/audits
        audits: {
            type: "string",
            multiple: true,
            default: ["largest-contentful-paint", "max-potential-fid", "cumulative-layout-shift", "server-response-time"],
        },
        "force-tracing": {
            type: "boolean",
            default: true,
        },
        interval: {
            type: "string",
            default: "60000",
        },
        port: {
            type: "string",
            default: "8080",
        }
    },
    allowPositionals: true,
    strict: true,
});

const SPAN_ID_BYTES = 8;
const TRACE_ID_BYTES = 16;
const SHARED_BUFFER = Buffer.allocUnsafe(TRACE_ID_BYTES);

function getIdGenerator(bytes) {
    return function generateId() {
        for (let i = 0; i < bytes / 4; i++) {
            SHARED_BUFFER.writeUInt32BE((Math.random() * 2 ** 32) >>> 0, i * 4);
        }

        for (let i = 0; i < bytes; i++) {
            if (SHARED_BUFFER[i] > 0) {
                break;
            } else if (i === bytes - 1) {
                SHARED_BUFFER[bytes - 1] = 1;
            }
        }

        return SHARED_BUFFER.toString("hex", 0, bytes);
    };
}

const exporter = new PrometheusExporter({
    startServer: true,
    port: port,
});
const meterProvider = new MeterProvider({
    readers: [exporter],
});

const hostMetrics = new HostMetrics({meterProvider});
hostMetrics.start();

const meter = meterProvider.getMeter("lighthouse-exporter");

const scoreGauge = meter.createGauge("lighthouse_score", {
    description: "Score of lighthouse results",
    labelKeys: ["category", "url", "form_factor"],
});
const auditGauge = meter.createGauge("lighthouse_audit", {
    description: "Audit of lighthouse results",
    labelKeys: ["id", "unit", "url", "form_factor"],
});
const errorsCounter = meter.createCounter("lighthouse_errors_total", {
    description: "Errors of lighthouse results",
    labelKeys: ["code", "url", "form_factor"],
});

class Mutex {
    constructor() {
        this.queue = [];
        this.locked = false;
    }

    lock() {
        return new Promise((resolve, reject) => {
            this.queue.push(() => {
                let once = false;
                const unlock = () => {
                    if (once) {
                        return;
                    }
                    once = true;
                    this.locked = false;
                    this._next();
                };
                resolve(unlock);
            });
            setImmediate(this._next.bind(this));
        });
    }

    _next() {
        if (!this.locked && this.queue.length > 0) {
            this.locked = true;
            let next = this.queue.shift();
            if (!next) {
                return;
            }

            next();
        }
    }
}

const generateSpanId = getIdGenerator(SPAN_ID_BYTES);
const generateTraceId = getIdGenerator(TRACE_ID_BYTES);

const lighthouseLoop = async (mutex, browser) => {
    for (const url of urls) {
        for (const formFactor of formFactors) {
            const unlock = await mutex.lock();

            const page = await browser.newPage();
            const session = await page.target().createCDPSession();
            await session.send("Network.clearBrowserCookies");
            await session.send("Network.clearBrowserCache");
            await page.close();

            const extraHeaders = {}
            if (forceTracing) {
                extraHeaders["traceparent"] = `00-${generateTraceId()}-${generateSpanId()}-01`;
            }
            console.log(JSON.stringify(Object.assign({url, formFactor}, extraHeaders)));

            await lighthouse(url, {
                port: new URL(browser.wsEndpoint()).port,
                output: "json",
                skipAudits: ["service-worker"],
                disableStorageReset: false,
                extraHeaders: extraHeaders,
            }, formFactor === "desktop" ? desktopConfig : undefined).then((results) => {
                const error = results.lhr.runtimeError;
                if (error !== undefined) {
                    console.error(JSON.stringify(Object.assign({
                        code: error.code,
                        message: error.message,
                        url,
                        formFactor,
                    }, extraHeaders)));
                    errorsCounter.add(1, {code: error.code, url, form_factor: formFactor});
                }

                Object.keys(results.lhr.categories).forEach((category) => {
                    const score = results.lhr.categories[category].score;
                    if (score !== null) {
                        scoreGauge.record(score * 100, {category, url, form_factor: formFactor});
                    }
                });

                audits.forEach((id) => {
                    const audit = results.lhr.audits[id];
                    if (audit !== undefined && audit.numericValue !== undefined) {
                        auditGauge.record(audit.numericValue, {
                            id,
                            unit: audit.numericUnit,
                            url,
                            form_factor: formFactor
                        });
                    }
                });
            });
            unlock();
        }
    }

    await new Promise((resolve) => setTimeout(resolve, interval));
    await lighthouseLoop(mutex, browser);
}

const mutex = new Mutex();

const browser = await puppeteer.launch({
    headless: true,
    args: [
        "--no-sandbox",
        "--disable-dev-shm-usage", // https://pptr.dev/troubleshooting#tips
    ]
});

await lighthouseLoop(mutex, browser)

await browser.close();
