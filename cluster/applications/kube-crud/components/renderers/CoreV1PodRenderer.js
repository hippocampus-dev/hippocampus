import { h } from "https://cdn.skypack.dev/preact@10.22.1";
import { useState } from "https://cdn.skypack.dev/preact@10.22.1/hooks";

import Terminal from "../Terminal.js";

const PodExec = ({ resource }) => {
  const containers = resource.spec?.containers ?? [];
  const defaultContainer =
    resource.metadata?.annotations?.[
      "kubectl.kubernetes.io/default-container"
    ] ??
    containers[0]?.name ??
    "";
  const [selectedContainer, setSelectedContainer] = useState(defaultContainer);

  return h("div", { class: "flex flex-col space-y-4 w-full" }, [
    h("div", { class: "flex flex-col" }, [
      h("label", { for: "container-select", class: "sr-only" }, "Container"),
      h(
        "select",
        {
          id: "container-select",
          class:
            "border border-blue-300 rounded px-4 py-2 bg-white text-blue-800",
          value: selectedContainer,
          onChange: (e) => setSelectedContainer(e.target.value),
        },
        containers.map((container) =>
          h("option", { value: container.name }, container.name),
        ),
      ),
    ]),
    selectedContainer !== "" &&
      h(Terminal, {
        namespace: resource.metadata.namespace,
        pod: resource.metadata.name,
        container: selectedContainer,
      }),
  ]);
};

export default class CoreV1PodRenderer {
  render(resource, editable, setEditable) {
    return h(PodExec, {
      key: resource.metadata.uid,
      resource,
    });
  }

  patch(resource, editable) {
    return {};
  }

  editable(resource) {
    return new Map();
  }
}
