import { defineEventHandler, getRouterParam, getQuery, createError } from "h3";
import { fetchView } from "@/lib/comfyui";

export default defineEventHandler(async (event) => {
  const filename = getRouterParam(event, "filename");
  if (!filename) {
    throw createError({ statusCode: 400, message: "filename is required" });
  }

  const query = getQuery(event);
  const subfolder = query.subfolder as string | undefined;
  const type = query.type as string | undefined;

  const response = await fetchView(filename, subfolder, type);
  if (!response.ok) {
    throw createError({
      statusCode: response.status,
      message: `ComfyUI /view failed: status=${response.status}`,
    });
  }

  const contentType = response.headers.get("content-type");
  if (contentType) {
    event.res.headers.set("Content-Type", contentType);
  }

  const arrayBuffer = await response.arrayBuffer();
  return Buffer.from(arrayBuffer);
});
