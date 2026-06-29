// added by create cloudflare to enable calling `getCloudflareContext()` in `next dev`
import { initOpenNextCloudflareForDev } from "@opennextjs/cloudflare";
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  typedRoutes: true,
  reactCompiler: true,
  turbopack: {
    rules: {
      "*": {
        condition: {
          query: /[?&]raw(?=&|$)/,
        },
        type: "raw",
      },
    },
  },
};

export default nextConfig;

initOpenNextCloudflareForDev();
