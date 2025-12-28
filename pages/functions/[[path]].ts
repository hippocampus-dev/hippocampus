import { handle } from "hono/cloudflare-pages";
import application from "../src/index";

export const onRequest = handle(application);
