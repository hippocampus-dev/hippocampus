import { h } from "https://cdn.skypack.dev/preact@10.22.1";
import {
  useEffect,
  useState,
} from "https://cdn.skypack.dev/preact@10.22.1/hooks";

const supportedGroups = [
  "snapshot.kaidotio.github.io",
  "skaffold.snapshot.kaidotio.github.io",
];
const supportedVersions = {
  "snapshot.kaidotio.github.io": ["v1"],
  "skaffold.snapshot.kaidotio.github.io": ["v1"],
};
const supportedKinds = {
  "snapshot.kaidotio.github.io": ["snapshot", "scheduledsnapshot"],
  "skaffold.snapshot.kaidotio.github.io": ["snapshot", "scheduledsnapshot"],
};

const App = () => {
  const [namespaces, setNamespaces] = useState(["default"]);
  const [selectedNamespace, setSelectedNamespace] = useState("default");
  const [groups, setGroups] = useState(supportedGroups);
  const [selectedGroup, setSelectedGroup] = useState(supportedGroups[0]);
  const [versions, setVersions] = useState(
    supportedVersions[selectedGroup] ?? [],
  );
  const [selectedVersion, setSelectedVersion] = useState("v1");
  const [kinds, setKinds] = useState(supportedKinds[selectedGroup] ?? []);
  const [selectedKind, setSelectedKind] = useState("snapshot");
  const [resources, setResources] = useState([]);
  const [selectedResource, setSelectedResource] = useState("");

  const [artifacts, setArtifacts] = useState(null);
  const [loading, setLoading] = useState(false);
  const [showDiff, setShowDiff] = useState(true);

  useEffect(() => {
    const searchParameters = new URLSearchParams(window.location.search);

    const namespaceParam = searchParameters.get("namespace");
    if (namespaceParam !== null) {
      setSelectedNamespace(namespaceParam);
    }

    const groupParam = searchParameters.get("group");
    if (groupParam !== null) {
      setSelectedGroup(groupParam);
    }

    const versionParam = searchParameters.get("version");
    if (versionParam !== null) {
      setSelectedVersion(versionParam);
    }

    const kindParam = searchParameters.get("kind");
    if (kindParam !== null) {
      setSelectedKind(kindParam);
    }

    const resourceParam = searchParameters.get("resource");
    if (resourceParam !== null) {
      setSelectedResource(resourceParam);
    }
  }, []);

  useEffect(() => {
    const abortController = new AbortController();

    fetch(`/api/`, {
      credentials: "include",
      signal: abortController.signal,
    })
      .then((response) => response.json())
      .then((json) => {
        const namespaces = json?.items?.map((item) => item.metadata.name);
        setNamespaces(namespaces);
      });

    return () => {
      abortController.abort();
    };
  }, []);

  useEffect(() => {
    setVersions(supportedVersions[selectedGroup] ?? []);
    setSelectedVersion(supportedVersions[selectedGroup][0]);

    setKinds(supportedKinds[selectedGroup] ?? []);
    setSelectedKind(supportedKinds[selectedGroup][0]);
  }, [selectedGroup]);

  useEffect(() => {
    const abortController = new AbortController();

    fetch(
      `/api/${selectedNamespace}/${selectedGroup}/${selectedVersion}/${selectedKind}`,
      {
        credentials: "include",
        signal: abortController.signal,
      },
    )
      .then((response) => {
        return response.json();
      })
      .then((json) => {
        const resources = json?.items?.map((item) => {
          return item.metadata.name;
        });
        setResources(resources);
      });

    return () => {
      abortController.abort();
    };
  }, [selectedNamespace, selectedGroup, selectedVersion, selectedKind]);

  useEffect(() => {
    if (resources.length === 0) {
      return;
    }

    setSelectedResource(resources[0]);
  }, [resources]);

  useEffect(() => {
    if (selectedResource === "") {
      return;
    }

    const abortController = new AbortController();
    setLoading(true);

    fetch(
      `/api/${selectedNamespace}/${selectedGroup}/${selectedVersion}/${selectedKind}/${selectedResource}/artifacts`,
      {
        credentials: "include",
        signal: abortController.signal,
      },
    )
      .then((response) => {
        return response.json();
      })
      .then((json) => {
        setArtifacts(json);
        setLoading(false);
      })
      .catch(() => {
        setLoading(false);
      });

    return () => {
      abortController.abort();
    };
  }, [
    selectedResource,
    selectedNamespace,
    selectedGroup,
    selectedVersion,
    selectedKind,
  ]);

  const renderScreenshot = (base64Data, alt) => {
    if (!base64Data)
      return h("div", { class: "text-gray-500" }, "画像がありません");
    return h("img", {
      src: `data:image/png;base64,${base64Data}`,
      alt: alt,
      class: "max-w-full h-auto border border-gray-300 rounded",
    });
  };

  const renderHTMLDiff = (base64Data) => {
    if (!base64Data)
      return h("div", { class: "text-gray-500" }, "HTMLの差分がありません");
    const htmlContent = atob(base64Data);
    return h("div", {
      class:
        "w-full h-96 border border-gray-300 rounded bg-white p-4 overflow-auto",
      dangerouslySetInnerHTML: { __html: htmlContent },
    });
  };

  return h(
    "div",
    {
      class: "flex flex-col min-h-screen items-center bg-blue-50 p-6 space-y-6",
    },
    [
      h(
        "div",
        { class: "flex flex-row space-x-6 bg-white p-6 rounded-lg shadow-lg" },
        [
          h("div", { class: "flex flex-col" }, [
            h(
              "label",
              { for: "namespace-select", class: "sr-only" },
              "Namespace",
            ),
            h(
              "select",
              {
                id: "namespace-select",
                class:
                  "border border-blue-300 rounded px-4 py-2 bg-white text-blue-800",
                value: selectedNamespace,
                onChange: (e) => setSelectedNamespace(e.target.value),
              },
              namespaces.map((namespace) =>
                h("option", { value: namespace }, namespace),
              ),
            ),
          ]),
          h("div", { class: "flex flex-col" }, [
            h("label", { for: "group-select", class: "sr-only" }, "API Group"),
            h(
              "select",
              {
                id: "group-select",
                class:
                  "border border-blue-300 rounded px-4 py-2 bg-white text-blue-800",
                value: selectedGroup,
                onChange: (e) => setSelectedGroup(e.target.value),
              },
              groups.map((group) => h("option", { value: group }, group)),
            ),
          ]),
          h("div", { class: "flex flex-col" }, [
            h(
              "label",
              { for: "version-select", class: "sr-only" },
              "API Version",
            ),
            h(
              "select",
              {
                id: "version-select",
                class:
                  "border border-blue-300 rounded px-4 py-2 bg-white text-blue-800",
                value: selectedVersion,
                onChange: (e) => setSelectedVersion(e.target.value),
              },
              versions.map((version) =>
                h("option", { value: version }, version),
              ),
            ),
          ]),
          h("div", { class: "flex flex-col" }, [
            h("label", { for: "kind-select", class: "sr-only" }, "Kind"),
            h(
              "select",
              {
                id: "kind-select",
                class:
                  "border border-blue-300 rounded px-4 py-2 bg-white text-blue-800",
                value: selectedKind,
                onChange: (e) => setSelectedKind(e.target.value),
              },
              kinds.map((kind) => h("option", { value: kind }, kind)),
            ),
          ]),
          h("div", { class: "flex flex-col" }, [
            h(
              "label",
              { for: "resource-select", class: "sr-only" },
              "Resource",
            ),
            h(
              "select",
              {
                id: "resource-select",
                class:
                  "border border-blue-300 rounded px-4 py-2 bg-white text-blue-800",
                value: selectedResource,
                onChange: (e) => setSelectedResource(e.target.value),
              },
              resources.map((resource) =>
                h("option", { value: resource }, resource),
              ),
            ),
          ]),
        ],
      ),
      loading
        ? h(
            "div",
            { class: "bg-white p-6 rounded-lg shadow-lg text-blue-800" },
            "読み込み中...",
          )
        : artifacts &&
          h(
            "div",
            {
              class: "bg-white p-6 rounded-lg shadow-lg w-full overflow-hidden",
            },
            [
              h("div", { class: "flex space-x-4 mb-4" }, [
                h(
                  "button",
                  {
                    class: `px-4 py-2 rounded ${showDiff ? "bg-blue-500 text-white" : "bg-gray-300 text-gray-700"}`,
                    onClick: () => setShowDiff(true),
                  },
                  "Diff表示",
                ),
                h(
                  "button",
                  {
                    class: `px-4 py-2 rounded ${!showDiff ? "bg-blue-500 text-white" : "bg-gray-300 text-gray-700"}`,
                    onClick: () => setShowDiff(false),
                  },
                  "Baseline/Target表示",
                ),
              ]),
              artifacts.diffRatio !== undefined &&
                h("div", { class: "mb-4" }, [
                  h(
                    "p",
                    { class: "text-sm text-gray-600" },
                    `画像差分: ${(artifacts.diffRatio * 100).toFixed(2)}%`,
                  ),
                ]),
              artifacts.htmlDiffRatio !== undefined &&
                h("div", { class: "mb-4" }, [
                  h(
                    "p",
                    { class: "text-sm text-gray-600" },
                    `HTML差分: ${(artifacts.htmlDiffRatio * 100).toFixed(2)}%`,
                  ),
                ]),
              showDiff
                ? h("div", { class: "space-y-6" }, [
                    h("div", null, [
                      h(
                        "h3",
                        { class: "text-lg font-semibold mb-2" },
                        "スクリーンショット差分",
                      ),
                      renderScreenshot(
                        artifacts.screenshotDiff,
                        "Screenshot Diff",
                      ),
                    ]),
                    h("div", null, [
                      h(
                        "h3",
                        { class: "text-lg font-semibold mb-2" },
                        "HTML差分",
                      ),
                      renderHTMLDiff(artifacts.htmlDiff),
                    ]),
                  ])
                : h("div", { class: "grid grid-cols-1 md:grid-cols-2 gap-6" }, [
                    h("div", null, [
                      h(
                        "h3",
                        { class: "text-lg font-semibold mb-2" },
                        "Baseline",
                      ),
                      renderScreenshot(
                        artifacts.baseline,
                        "Baseline Screenshot",
                      ),
                      artifacts.baselineHtml &&
                        h("div", { class: "mt-4" }, [
                          h(
                            "h4",
                            { class: "text-md font-semibold mb-2" },
                            "Baseline HTML",
                          ),
                          h(
                            "div",
                            {
                              class:
                                "w-full h-64 border border-gray-300 rounded bg-gray-50 p-2 overflow-auto",
                            },
                            h(
                              "pre",
                              { class: "text-xs" },
                              atob(artifacts.baselineHtml),
                            ),
                          ),
                        ]),
                    ]),
                    h("div", null, [
                      h(
                        "h3",
                        { class: "text-lg font-semibold mb-2" },
                        "Target",
                      ),
                      renderScreenshot(artifacts.target, "Target Screenshot"),
                      artifacts.targetHtml &&
                        h("div", { class: "mt-4" }, [
                          h(
                            "h4",
                            { class: "text-md font-semibold mb-2" },
                            "Target HTML",
                          ),
                          h(
                            "div",
                            {
                              class:
                                "w-full h-64 border border-gray-300 rounded bg-gray-50 p-2 overflow-auto",
                            },
                            h(
                              "pre",
                              { class: "text-xs" },
                              atob(artifacts.targetHtml),
                            ),
                          ),
                        ]),
                    ]),
                  ]),
            ],
          ),
    ],
  );
};

export default App;
