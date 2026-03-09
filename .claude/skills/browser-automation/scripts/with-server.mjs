#!/usr/bin/env node
/**
 * Start one or more servers, wait for them to be ready, run a command, then clean up.
 *
 * Usage:
 *   # Single server
 *   node scripts/with-server.mjs --server "npm run dev" --port 5173 -- node automation.mjs
 *   node scripts/with-server.mjs --server "npm start" --port 3000 -- node test.mjs
 *
 *   # Multiple servers
 *   node scripts/with-server.mjs \
 *     --server "cd backend && python server.py" --port 3000 \
 *     --server "cd frontend && npm run dev" --port 5173 \
 *     -- node test.mjs
 */

import {parseArgs} from "node:util";
import {spawn} from "node:child_process";
import {createConnection} from "node:net";

const {
    values: {server: servers, port: ports, timeout, help},
    positionals: command,
} = parseArgs({
    options: {
        server: {
            type: "string",
            multiple: true,
            short: "s",
        },
        port: {
            type: "string",
            multiple: true,
            short: "p",
        },
        timeout: {
            type: "string",
            default: "30000",
            short: "t",
        },
        help: {
            type: "boolean",
            short: "h",
        },
    },
    allowPositionals: true,
    strict: true,
});

if (help) {
    console.log(`Usage: node with-server.mjs [options] -- <command>

Options:
  -s, --server <cmd>   Server command to run (can be repeated for multiple servers)
  -p, --port <port>    Port to wait for (must match --server count)
  -t, --timeout <ms>   Timeout in milliseconds per server (default: 30000)
  -h, --help           Show this help message

Examples:
  # Single server
  node with-server.mjs --server "npm run dev" --port 5173 -- node test.mjs

  # Multiple servers
  node with-server.mjs \\
    --server "cd backend && python server.py" --port 3000 \\
    --server "cd frontend && npm run dev" --port 5173 \\
    -- node test.mjs
`);
    process.exit(0);
}

if (!servers || servers.length === 0) {
    console.error("Error: At least one --server is required");
    process.exit(1);
}

if (!ports || ports.length === 0) {
    console.error("Error: At least one --port is required");
    process.exit(1);
}

if (servers.length !== ports.length) {
    console.error("Error: Number of --server and --port arguments must match");
    process.exit(1);
}

if (command.length === 0) {
    console.error("Error: No command specified to run");
    process.exit(1);
}

const timeoutMilliseconds = parseInt(timeout, 10);
if (isNaN(timeoutMilliseconds) || timeoutMilliseconds <= 0) {
    console.error("Error: Invalid timeout. Must be a positive number.");
    process.exit(1);
}

for (let i = 0; i < ports.length; i++) {
    const port = parseInt(ports[i], 10);
    if (isNaN(port) || port < 1 || port > 65535) {
        console.error(`Error: Invalid port "${ports[i]}". Must be between 1 and 65535.`);
        process.exit(1);
    }
}

function isServerReady(port, timeoutMilliseconds) {
    return new Promise((resolve) => {
        const startTime = Date.now();

        function tryConnect() {
            const socket = createConnection({port, host: "localhost"}, () => {
                socket.destroy();
                resolve(true);
            });

            socket.on("error", () => {
                socket.destroy();
                if (Date.now() - startTime < timeoutMilliseconds) {
                    setTimeout(tryConnect, 500);
                } else {
                    resolve(false);
                }
            });

            socket.setTimeout(1000, () => {
                socket.destroy();
                if (Date.now() - startTime < timeoutMilliseconds) {
                    setTimeout(tryConnect, 500);
                } else {
                    resolve(false);
                }
            });
        }

        tryConnect();
    });
}

const serverProcesses = [];

async function cleanup() {
    console.log(`\nStopping ${serverProcesses.length} server(s)...`);
    for (let i = 0; i < serverProcesses.length; i++) {
        const serverProcess = serverProcesses[i];
        if (serverProcess && !serverProcess.killed) {
            serverProcess.kill("SIGTERM");
            await new Promise((resolve) => setTimeout(resolve, 100));
            if (!serverProcess.killed) {
                serverProcess.kill("SIGKILL");
            }
            console.log(`Server ${i + 1} stopped`);
        }
    }
    console.log("All servers stopped");
}

process.on("SIGINT", async () => {
    await cleanup();
    process.exit(130);
});

process.on("SIGTERM", async () => {
    await cleanup();
    process.exit(143);
});

try {
    for (let i = 0; i < servers.length; i++) {
        const serverCommand = servers[i];
        const port = parseInt(ports[i], 10);

        console.log(`Starting server ${i + 1}/${servers.length}: ${serverCommand}`);

        const serverProcess = spawn(serverCommand, [], {
            shell: true,
            stdio: ["ignore", "pipe", "pipe"],
        });

        serverProcesses.push(serverProcess);

        serverProcess.stdout.on("data", (data) => {
            process.stdout.write(`[server ${i + 1}] ${data}`);
        });

        serverProcess.stderr.on("data", (data) => {
            process.stderr.write(`[server ${i + 1}] ${data}`);
        });

        console.log(`Waiting for server on port ${port}...`);
        const ready = await isServerReady(port, timeoutMilliseconds);

        if (!ready) {
            throw new Error(`Server failed to start on port ${port} within ${timeoutMilliseconds}ms`);
        }

        console.log(`Server ready on port ${port}`);
    }

    console.log(`\nAll ${servers.length} server(s) ready`);
    console.log(`Running: ${command.join(" ")}\n`);

    const result = spawn(command[0], command.slice(1), {
        stdio: "inherit",
        shell: false,
    });

    result.on("close", async (code) => {
        await cleanup();
        process.exit(code ?? 0);
    });

    result.on("error", async (error) => {
        console.error("Command failed:", error);
        await cleanup();
        process.exit(1);
    });
} catch (error) {
    console.error("Error:", error);
    await cleanup();
    process.exit(1);
}
