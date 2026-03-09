import {h} from "https://cdn.skypack.dev/preact@10.22.1";
import {useEffect, useState} from "https://cdn.skypack.dev/preact@10.22.1/hooks";

import {HOST} from "../constants/host.js";

class DefaultRenderer {
    render(resource, editable, setEditable) {
        return h("div", {class: "bg-white p-6 rounded-lg shadow-lg text-blue-800 w-full overflow-hidden break-all"}, [
            h("p", {}, "This resource is not supported."),
        ]);
    }

    patch(resource, editable) {
        return {};
    }

    editable(resource) {
        return new Map();
    }
}

class BatchV1CronJobRenderer {
    render(resource, editable, setEditable) {
        const containers = resource.spec?.jobTemplate?.spec?.template?.spec?.containers ?? [];

        return h("div", {class: "bg-white p-6 rounded-lg shadow-lg text-blue-800 w-full space-y-4 overflow-hidden break-all"}, [
            ...containers.map((container) => (
                h("div", {key: container.name}, [
                    h("h3", null, container.name),
                    h("table", {class: "table-auto w-full text-left"}, [
                        h("thead", null, [
                            h("tr", null, [
                                h("th", {class: "px-4 py-2"}, "Name"),
                                h("th", {class: "px-4 py-2"}, "Value")
                            ])
                        ]),
                        h("tbody", null,
                            (editable.get(container.name) ?? []).map((env, envIndex) => (
                                h("tr", {key: envIndex}, [
                                    h("td", {class: "border px-4 py-2"}, env.name),
                                    h("td", {class: "border px-4 py-2"},
                                        h("input", {
                                            type: "text",
                                            "aria-label": `Value for ${env.name}`,
                                            class: "border border-blue-300 rounded px-2 py-1 bg-white text-blue-800 w-full",
                                            value: env.value ?? "",
                                            onkeyup: (e) => {
                                                const newEnv = [...editable.get(container.name)];
                                                newEnv[envIndex].value = e.target.value;

                                                const editableMap = new Map(editable);
                                                editableMap.set(container.name, newEnv);
                                                setEditable(editableMap);
                                            }
                                        })
                                    )
                                ])
                            ))
                        )
                    ])
                ])
            ))
        ]);
    }

    patch(resource, editable) {
        const p = {
            "spec": {
                "jobTemplate": {
                    "spec": {
                        "template": {
                            "spec": {
                                "$setElementOrder/containers": resource?.spec?.jobTemplate?.spec?.template?.spec?.containers.map((container) => {
                                    return {name: container.name};
                                }) ?? [],
                                "containers": []
                            }
                        }
                    }
                }
            }
        };

        editable.forEach((envs, containerName) => {
            p.spec.jobTemplate.spec.template.spec.containers.push({
                "name": containerName,
                "env": envs,
            });
        });

        return p;
    }

    editable(resource) {
        const containers = resource.spec?.jobTemplate?.spec?.template?.spec?.containers ?? [];

        const editableMap = new Map();
        containers.forEach((container) => {
            editableMap.set(container.name, container.env ?? []);
        });
        return editableMap;
    }
}

const rendererFactory = (group, version, kind) => {
    if (group === "batch" && version === "v1" && kind === "CronJob") {
        return new BatchV1CronJobRenderer();
    }

    return new DefaultRenderer();
}

const supportedGroups = ["batch"];
const supportedVersions = {
    "batch": ["v1"],
};
const supportedKinds = {
    "batch": ["CronJob"],
};

const App = () => {
    const [namespaces, setNamespaces] = useState(["default"]);
    const [selectedNamespace, setSelectedNamespace] = useState("default");
    const [groups, setGroups] = useState(supportedGroups);
    const [selectedGroup, setSelectedGroup] = useState("batch");
    const [versions, setVersions] = useState(supportedVersions[selectedGroup] ?? []);
    const [selectedVersion, setSelectedVersion] = useState("v1");
    const [kinds, setKinds] = useState(supportedKinds[selectedGroup] ?? []);
    const [selectedKind, setSelectedKind] = useState("CronJob");
    const [resources, setResources] = useState([]);
    const [selectedResource, setSelectedResource] = useState("");

    const [resource, setResource] = useState({});
    const [editable, setEditable] = useState(new Map());
    const [originalEditable, setOriginalEditable] = useState("{}");
    const [isEditMode, setIsEditMode] = useState(false);
    const [updateDisabled, setupdateDisabled] = useState(true);

    useEffect(() => {
        const params = new URLSearchParams(window.location.search);

        const namespaceParam = params.get("namespace");
        if (namespaceParam !== null) {
            setSelectedNamespace(namespaceParam);
        }

        const groupParam = params.get("group");
        if (groupParam !== null) {
            setSelectedGroup(groupParam);
        }

        const versionParam = params.get("version");
        if (versionParam !== null) {
            setSelectedVersion(versionParam);
        }

        const kindParam = params.get("kind");
        if (kindParam !== null) {
            setSelectedKind(kindParam);
        }

        const resourceParam = params.get("resource");
        if (resourceParam !== null) {
            setSelectedResource(resourceParam);
        }
    }, []);

    useEffect(() => {
        const abortController = new AbortController();

        fetch(`${HOST}/`, {
            credentials: "include",
            signal: abortController.signal,
        }).then((response) => response.json()).then((json) => {
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

        fetch(`${HOST}/${selectedNamespace}/${selectedGroup}/${selectedVersion}/${selectedKind.toLowerCase()}`, {
            credentials: "include",
            signal: abortController.signal,
        }).then((response) => {
            return response.json();
        }).then((json) => {
            const resources = json?.items?.map((item) => {
                return item.metadata.name;
            });
            setResources(resources);
        });

        return () => {
            abortController.abort();
        }
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

        fetch(`${HOST}/${selectedNamespace}/${selectedGroup}/${selectedVersion}/${selectedKind.toLowerCase()}/${selectedResource}`, {
            credentials: "include",
            signal: abortController.signal,
        }).then((response) => {
            return response.json();
        }).then((json) => {
            setResource(json);
            const editable = rendererFactory(selectedGroup, selectedVersion, selectedKind).editable(json);
            setEditable(editable);
            setOriginalEditable(JSON.stringify([...editable]));
        });

        return () => {
            abortController.abort();
        }
    }, [selectedResource]);

    useEffect(() => {
        const isDisabled = JSON.stringify([...editable]) === originalEditable;
        setupdateDisabled(isDisabled);
    }, [editable, originalEditable]);

    const handleCreateJob = () => {
        const patch = rendererFactory(selectedGroup, selectedVersion, selectedKind).patch(resource, editable);

        if (!window.confirm("Do you really want to create a job from this cronjob?")) {
            return;
        }

        const newPatch = {
            ...patch?.spec.jobTemplate,
        };

        fetch(`${HOST}/${selectedNamespace}/${selectedGroup}/${selectedVersion}/job/manual-${selectedResource}-${Date.now()}/from/cronjob/${selectedResource}`, {
            method: "POST",
            headers: {
                "Content-Type": "application/strategic-merge-patch+json",
            },
            body: JSON.stringify(newPatch),
            credentials: "include",
        }).then((response) => {
            return response.json();
        }).then((json) => {
            setIsEditMode(false)
        });
    };

    const handleUpdate = () => {
        const patch = rendererFactory(selectedGroup, selectedVersion, selectedKind).patch(resource, editable);

        if (!window.confirm("Do you really want to update this resource?")) {
            return;
        }

        fetch(`${HOST}/${selectedNamespace}/${selectedGroup}/${selectedVersion}/${selectedKind.toLowerCase()}/${selectedResource}`, {
            method: "PATCH",
            headers: {
                "Content-Type": "application/strategic-merge-patch+json",
            },
            body: JSON.stringify(patch),
            credentials: "include",
        }).then((response) => {
            return response.json();
        }).then((json) => {
            setIsEditMode(false)
        });
    };

    const handleDelete = () => {
        if (!window.confirm("Do you really want to delete this resource?")) {
            return;
        }

        fetch(`${HOST}/${selectedNamespace}/${selectedGroup}/${selectedVersion}/${selectedKind.toLowerCase()}/${selectedResource}`, {
            method: "DELETE",
            credentials: "include",
        }).then((response) => {
            return response.json();
        }).then((json) => {
            setIsEditMode(false)
        });
    }

    return (
        h("div", {class: "flex flex-col min-h-screen items-center bg-blue-50 p-6 space-y-6"}, [
            h("div", {class: "flex flex-row space-x-6 bg-white p-6 rounded-lg shadow-lg"}, [
                h("div", {class: "flex flex-col"}, [
                    h("label", {for: "namespace-select", class: "sr-only"}, "Namespace"),
                    h("select", {
                        id: "namespace-select",
                        class: "border border-blue-300 rounded px-4 py-2 bg-white text-blue-800",
                        value: selectedNamespace,
                        onChange: (e) => setSelectedNamespace(e.target.value),
                    }, namespaces.map((namespace) => h("option", {value: namespace}, namespace))),
                ]),
                h("div", {class: "flex flex-col"}, [
                    h("label", {for: "group-select", class: "sr-only"}, "API Group"),
                    h("select", {
                        id: "group-select",
                        class: "border border-blue-300 rounded px-4 py-2 bg-white text-blue-800",
                        value: selectedGroup,
                        onChange: (e) => setSelectedGroup(e.target.value),
                    }, groups.map((group) => h("option", {value: group}, group))),
                ]),
                h("div", {class: "flex flex-col"}, [
                    h("label", {for: "version-select", class: "sr-only"}, "API Version"),
                    h("select", {
                        id: "version-select",
                        class: "border border-blue-300 rounded px-4 py-2 bg-white text-blue-800",
                        value: selectedVersion,
                        onChange: (e) => setSelectedVersion(e.target.value),
                    }, versions.map((version) => h("option", {value: version}, version))),
                ]),
                h("div", {class: "flex flex-col"}, [
                    h("label", {for: "kind-select", class: "sr-only"}, "Kind"),
                    h("select", {
                        id: "kind-select",
                        class: "border border-blue-300 rounded px-4 py-2 bg-white text-blue-800",
                        value: selectedKind,
                        onChange: (e) => setSelectedKind(e.target.value),
                    }, kinds.map((kind) => h("option", {value: kind}, kind))),
                ]),
                h("div", {class: "flex flex-col"}, [
                    h("label", {for: "resource-select", class: "sr-only"}, "Resource"),
                    h("select", {
                        id: "resource-select",
                        class: "border border-blue-300 rounded px-4 py-2 bg-white text-blue-800",
                        value: selectedResource,
                        onChange: (e) => setSelectedResource(e.target.value),
                    }, resources.map((resource) => h("option", {value: resource}, resource))),
                ]),
            ]),
            !isEditMode ? (
                h("div", {class: "bg-white p-6 rounded-lg shadow-lg text-blue-800 w-full overflow-hidden break-all"}, [
                    h("button", {
                        class: "border border-blue-300 rounded px-4 py-2 bg-blue-500 text-white mb-4",
                        onClick: () => setIsEditMode(true)
                    }, "Edit"),
                    h("pre", {class: "whitespace-pre-wrap"}, JSON.stringify(resource, null, 2)),
                ])
            ) : (
                h("div", {class: "bg-white p-6 rounded-lg shadow-lg text-blue-800 w-full space-y-4 overflow-hidden break-all"}, [
                    rendererFactory(selectedGroup, selectedVersion, selectedKind).render(resource, editable, setEditable),
                    selectedKind === "CronJob" && h("button", {
                        class: "border border-blue-300 rounded px-4 py-2 mt-4 bg-green-500 text-white",
                        onClick: handleCreateJob,
                    }, "Create Job"),
                    h("button", {
                        class: `border border-blue-300 rounded px-4 py-2 mt-4 ${updateDisabled ? "bg-gray-300 text-gray-700 cursor-not-allowed" : "bg-blue-500 text-white"}`,
                        onClick: handleUpdate,
                        disabled: updateDisabled
                    }, "Update"),
                    h("button", {
                        class: "border border-blue-300 rounded px-4 py-2 bg-red-500 text-white mt-4",
                        onClick: handleDelete,
                    }, "Delete"),
                    h("button", {
                        class: "border border-blue-300 rounded px-4 py-2 bg-gray-500 text-white mt-4",
                        onClick: () => setIsEditMode(false)
                    }, "Close")
                ])
            )
        ])
    );
};

export default App;
