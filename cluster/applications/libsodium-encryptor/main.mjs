import express from "express";
import sodium from "tweetsodium";

const app = express();

app.use(express.json());

app.post("/", (request, response) => {
  const key = request.body.key;
  const value = request.body.value;
  if (key === undefined || value === undefined) {
    response.status(400);
    response.send("Bad Request");
    return;
  }

  try {
    const messageBytes = Buffer.from(value);
    const keyBytes = Buffer.from(key, "base64");
    const encryptedBytes = sodium.seal(messageBytes, keyBytes);
    const encrypted = Buffer.from(encryptedBytes).toString("base64");

    response.send(encrypted);
  } catch (e) {
    if (e.message === "bad public key size") {
      response.status(400);
      response.send("Bad Request");
      return;
    }

    console.error(e);
    response.status(500);
    response.send("Internal Server Error");
  }
});

app.get("/healthz", (request, response) => {
  response.send("OK");
});

const server = app.listen(8080);

process.on("SIGTERM", () => {
  server.close();
});
