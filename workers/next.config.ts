import type {NextConfig} from "next";
// added by create cloudflare to enable calling `getCloudflareContext()` in `next dev`
import {initOpenNextCloudflareForDev} from '@opennextjs/cloudflare';

const nextConfig: NextConfig = {
    /* config options here */
};

export default nextConfig;

initOpenNextCloudflareForDev();
