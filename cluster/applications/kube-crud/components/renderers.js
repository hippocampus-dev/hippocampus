import DefaultRenderer from "./renderers/DefaultRenderer.js";
import BatchV1CronJobRenderer from "./renderers/BatchV1CronJobRenderer.js";
import CoreV1PodRenderer from "./renderers/CoreV1PodRenderer.js";

export const rendererFactory = (group, version, kind) => {
  if (group === "batch" && version === "v1" && kind === "CronJob") {
    return new BatchV1CronJobRenderer();
  }

  if (group === "core" && version === "v1" && kind === "Pod") {
    return new CoreV1PodRenderer();
  }

  return new DefaultRenderer();
};

export const supportedGroups = ["batch", "core"];

export const supportedVersions = {
  batch: ["v1"],
  core: ["v1"],
};

export const supportedKinds = {
  batch: ["CronJob"],
  core: ["Pod"],
};
