import { h } from "https://cdn.skypack.dev/preact@10.22.1";

const CronJobEnvEditor = ({ resource, editable, setEditable }) => {
  const containers =
    resource.spec?.jobTemplate?.spec?.template?.spec?.containers ?? [];

  return h(
    "div",
    {
      class:
        "bg-white p-6 rounded-lg shadow-lg text-blue-800 w-full space-y-4 overflow-hidden break-all",
    },
    [
      ...containers.map((container) =>
        h("div", { key: container.name }, [
          h("h3", null, container.name),
          h("table", { class: "table-auto w-full text-left" }, [
            h("thead", null, [
              h("tr", null, [
                h("th", { scope: "col", class: "px-4 py-2" }, "Name"),
                h("th", { scope: "col", class: "px-4 py-2" }, "Value"),
              ]),
            ]),
            h(
              "tbody",
              null,
              (editable.get(container.name) ?? []).map(
                (environmentVariable, environmentVariableIndex) =>
                  h("tr", { key: environmentVariableIndex }, [
                    h(
                      "td",
                      { class: "border px-4 py-2" },
                      environmentVariable.name,
                    ),
                    h(
                      "td",
                      { class: "border px-4 py-2" },
                      h("input", {
                        type: "text",
                        "aria-label": `Value for ${environmentVariable.name}`,
                        class:
                          "border border-blue-300 rounded px-2 py-1 bg-white text-blue-800 w-full",
                        value: environmentVariable.value ?? "",
                        onkeyup: (e) => {
                          const newEnvironmentVariables = [
                            ...editable.get(container.name),
                          ];
                          newEnvironmentVariables[
                            environmentVariableIndex
                          ].value = e.target.value;

                          const editableMap = new Map(editable);
                          editableMap.set(
                            container.name,
                            newEnvironmentVariables,
                          );
                          setEditable(editableMap);
                        },
                      }),
                    ),
                  ]),
              ),
            ),
          ]),
        ]),
      ),
    ],
  );
};

export default class BatchV1CronJobRenderer {
  render(resource, editable, setEditable) {
    return h(CronJobEnvEditor, { resource, editable, setEditable });
  }

  patch(resource, editable) {
    const p = {
      spec: {
        jobTemplate: {
          spec: {
            template: {
              spec: {
                "$setElementOrder/containers":
                  resource?.spec?.jobTemplate?.spec?.template?.spec?.containers.map(
                    (container) => {
                      return { name: container.name };
                    },
                  ) ?? [],
                containers: [],
              },
            },
          },
        },
      },
    };

    editable.forEach((envs, containerName) => {
      p.spec.jobTemplate.spec.template.spec.containers.push({
        name: containerName,
        env: envs,
      });
    });

    return p;
  }

  editable(resource) {
    const containers =
      resource.spec?.jobTemplate?.spec?.template?.spec?.containers ?? [];

    const editableMap = new Map();
    containers.forEach((container) => {
      editableMap.set(container.name, container.env ?? []);
    });
    return editableMap;
  }
}
