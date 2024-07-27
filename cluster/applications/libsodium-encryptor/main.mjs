import sodium from "tweetsodium";
import express from "express";
import bodyParser from "body-parser";

const app = express();

app.use(bodyParser.json());

app.post("/", (req, res) => {
    const key = req.body.key;
    const value = req.body.value;
    if (key === undefined || value === undefined) {
        res.status(400);
        res.send("Bad Request");
        return;
    }

    try {
        const messageBytes = Buffer.from(value);
        const keyBytes = Buffer.from(key, "base64");
        const encryptedBytes = sodium.seal(messageBytes, keyBytes);
        const encrypted = Buffer.from(encryptedBytes).toString("base64");

        res.send(encrypted);
    } catch (e) {
        if (e.message === "bad public key size") {
            res.status(400);
            res.send("Bad Request");
            return;
        }

        console.error(e);
        res.status(500);
        res.send("Internal Server Error");
    }
});

app.get("/health", (req, res) => {
    res.send("OK");
});

const server = app.listen(8080);

process.on("SIGTERM", () => {
    server.close();
});
