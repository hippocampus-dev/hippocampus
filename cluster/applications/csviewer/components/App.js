import {h} from "https://cdn.skypack.dev/preact@10.22.1";
import {useEffect, useMemo, useState} from "https://cdn.skypack.dev/preact@10.22.1/hooks";

// https://datatracker.ietf.org/doc/html/rfc4180#section-2
// If fields are not enclosed with double quotes, then double quotes may not appear inside the fields.
class RFC4180DoubleQuoteError extends Error {
    constructor() {
        super("the field containing double quotes must be enclosed in double quotes");
        this.name = "RFC4180DoubleQuoteError";
    }
}

// https://datatracker.ietf.org/doc/html/rfc4180
const parseRFC4180 = (csv) => {
    const rows = []

    const newLineRegexp = /\r\n|\r/g;
    const newCSV = csv.replace(newLineRegexp, "\n");
    const lineEndCharacter = "\n";

    let row = [];
    let i = 0;
    while (i <= newCSV.length) {
        let commaIndex = -1;
        let lineEndIndex = -1;

        if (newCSV.charAt(i) === '"') {
            let quoteIndex = i + 1;

            while (quoteIndex <= newCSV.length) {
                quoteIndex = newCSV.indexOf('"', quoteIndex);

                if (quoteIndex === -1) {
                    throw new RFC4180DoubleQuoteError();
                }

                const nextChar = newCSV.charAt(++quoteIndex);
                // https://datatracker.ietf.org/doc/html/rfc4180#section-2
                // If double-quotes are used to enclose fields, then a double-quote appearing inside a field must be escaped by preceding it with another double quote.
                if (nextChar === '"') {
                    quoteIndex++;
                } else if (nextChar === "," || nextChar === lineEndCharacter || quoteIndex === newCSV.length) {
                    break;
                } else {
                    throw new RFC4180DoubleQuoteError();
                }
            }

            const next = quoteIndex;
            const field = newCSV.slice(i + 1, next - 1).replace(/""/g, '"');
            row.push(field);
            i = next;
        } else {
            commaIndex = newCSV.indexOf(",", i)
            commaIndex = commaIndex < 0 ? newCSV.length : commaIndex;
            lineEndIndex = newCSV.indexOf(lineEndCharacter, i);
            lineEndIndex = lineEndIndex < 0 ? newCSV.length : lineEndIndex;

            const next = Math.min(commaIndex, lineEndIndex);
            const field = newCSV.slice(i, next);
            if (field.includes('"')) {
                throw new RFC4180DoubleQuoteError();
            }
            row.push(field);
            i = next;
        }

        if (i === newCSV.length) {
            // https://datatracker.ietf.org/doc/html/rfc4180#section-2
            // The last record in the file may or may not have an ending line break.
            if (!(row.length === 1 && row[0] === "")) {
                rows.push(row);
            }
            break;
        } else if (lineEndCharacter === newCSV.slice(i, i + lineEndCharacter.length)) {
            rows.push(row);
            row = [];
            i += lineEndCharacter.length;
        } else {
            i++;
        }
    }

    return rows;
};

const markdownLinkRegexp = /\[([^\]]+)\]\(([^\)]+)\)/g;
const markdownLinkify = (text) => {
    const parts = text.split(markdownLinkRegexp);
    const elements = [];
    for (let i = 0; i < parts.length; i++) {
        const part = parts[i];
        if (i % 3 === 0) {
            elements.push(part);
        } else if (i % 3 === 1) {
            elements.push(h("a", {
                href: parts[++i],
                target: "_blank",
                class: "underline hover:text-orange-800 focus:outline-none focus:text-orange-800"
            }, part));
        }
    }
    return elements;
}

const scrollToID = (id) => {
    const element = document.getElementById(id);
    if (element) {
        element.scrollIntoView({behavior: "smooth"});
    }
};

const segmenter = new Intl.Segmenter("ja");
const bitap = (text, pattern, maxDistance = 0) => {
    const patternLength = [...segmenter.segment(pattern)].length
    const textLength = [...segmenter.segment(text)].length;

    if (patternLength === 0) {
        return 0;
    }

    if (maxDistance < 0) {
        throw new Error("maxDistance must be >= 0");
    }

    if (patternLength > 64) {
        throw new Error("pattern is too long");
    }

    let patternMask = new Map();
    for (let i = 0; i < patternLength; i++) {
        const c = pattern.codePointAt(i);
        if (!patternMask.has(c)) {
            patternMask.set(c, ~0);
        }
        patternMask.set(c, patternMask.get(c) & ~(1 << i));
    }

    const state = new Array(maxDistance + 1).fill(~1);

    for (let i = 0; i < textLength; i++) {
        const c = text.codePointAt(i);

        let oldState = state[0];
        state[0] |= patternMask.get(c) ?? ~0;
        state[0] <<= 1;

        for (let d = 1; d <= maxDistance; d++) {
            const tmp = state[d];
            state[d] |= patternMask.get(c) ?? ~0;
            state[d] &= oldState;
            state[d] <<= 1;
            oldState = tmp;
        }

        if ((state[maxDistance] & (1 << patternLength)) === 0) {
            return i - patternLength + 1;
        }
    }

    return -1;
};

const App = ({files}) => {
    const [selectedFile, setSelectedFile] = useState(Object.keys(files).shift());
    const [hash, setHash] = useState("");
    const [rows, setRows] = useState([]);
    const [query, setQuery] = useState("");

    useEffect(() => {
        const params = new URLSearchParams(window.location.search);
        const fileParam = params.get("file");
        if (files[fileParam]) {
            setSelectedFile(fileParam);
        }

        const hash = window.location.hash;
        if (hash) {
            setHash(hash.slice(1));
        }
    }, []);

    useEffect(() => {
        if (!selectedFile) {
            return;
        }
        const params = new URLSearchParams(window.location.search);
        params.set("file", selectedFile);
        window.history.replaceState({}, "", `${window.location.pathname}?${params.toString()}`);

        const abortController = new AbortController();

        fetch(selectedFile, {
            signal: abortController.signal,
        }).then((response) => {
            return response.text();
        }).then((text) => {
            setRows(parseRFC4180(text));
        });

        return () => {
            abortController.abort();
        };
    }, [selectedFile]);

    useEffect(() => {
        if (!hash) {
            return;
        }
        window.location.hash = hash;

        scrollToID(hash);

        return () => {
            window.location.hash = "";
        };
    }, [hash, rows]);

    const URLButton = ({file, text}) => {
        const classes = "flex-grow rounded-md font-semibold focus:outline-none focus:ring-4 focus:ring-orange-600 focus:ring-opacity-100 focus:text-orange-600 focus:bg-white py-4 px-2";
        return h("button", {
            onClick: () => {
                setSelectedFile(file);
                setHash("");
            },
            onPointerOver: (event) => {
                if (!document.head.querySelector(`link[href="${file}"]`)) {
                    const link = document.createElement("link");
                    link.rel = "prefetch";
                    link.href = file;
                    document.head.appendChild(link);
                }
            },
            class: file === selectedFile ? [classes, "text-orange-600 bg-white ring-4 ring-orange-600 ring-opacity-100 hover:bg-white"].join(" ") : [classes, "text-white bg-orange-600 hover:bg-orange-500"].join(" "),
        }, text);
    };

    const [rawHeader, ...body] = rows;

    const {header, headerPTR} = useMemo(() => {
        const preferredLanguages = new Set(navigator.languages.map((language) => language.split("-")[0]));
        const header = [];
        const headerPTR = new Map();
        rawHeader?.forEach((column, index) => {
            const separatorIndex = column.lastIndexOf(":");
            if (separatorIndex !== -1 && preferredLanguages.has(column.slice(separatorIndex + 1))) {
                return;
            }
            header.push(column);

            let resolvedIndex = index;
            for (const language of preferredLanguages) {
                const localizedIndex = rawHeader?.indexOf(`${column}:${language}`);
                if (localizedIndex !== -1) {
                    resolvedIndex = localizedIndex;
                    break;
                }
            }
            headerPTR.set(column, resolvedIndex);
        });
        return {header, headerPTR};
    }, [rawHeader]);

    const storedColumns = useMemo(() => {
            return header?.filter((column) => {
                return files[selectedFile]?.mapping?.[column]?.store ?? true;
            })
        },
        [header, selectedFile, files],
    );
    const indexedColumns = useMemo(() => {
            return header?.filter((column) => {
                return files[selectedFile]?.mapping?.[column]?.index ?? true;
            })
        },
        [header, selectedFile, files],
    );

    return (
        h("div", {class: "flex flex-col items-center bg-gray-100 p-6 space-y-4"}, [
            h("div", {class: "w-full lg:w-1/2 mt-4"}, [
                h("div", {class: "flex flex-row overflow-x-auto mb-4 py-4 px-4 space-x-2"}, Object.keys(files).map((file) => h(URLButton, {
                    file,
                    text: files[file]?.alias ?? file
                }))),
                h("label", {for: "search-box", class: "sr-only"}, "Search for given file"),
                h("input", {
                    type: "text",
                    value: query,
                    placeholder: "Search for given file",
                    id: "search-box",
                    onInput: (event) => setQuery(event.target.value),
                    autofocus: true,
                    class: "w-full rounded-md border-4 border-orange-600 focus:ring-4 focus:ring-orange-600 p-4"
                }),
            ]),
            h("table", {class: "min-w-full"}, [
                h("thead", {class: "text-white bg-orange-700"}, [
                    h("tr", {}, storedColumns?.map((column) => {
                        return h("th", {
                            scope: "col",
                            class: `text-left px-6 py-3 w-1/${storedColumns.length}`
                        }, column);
                    })),
                ]),
                h("tbody", {class: "bg-white divide-y divide-orange-600"}, (() => {
                    const terms = query.split(" ").map((term) => term.trim().toLowerCase());
                    return body?.filter((row) => {
                        return terms.every((term) => {
                            return indexedColumns?.some((column) => {
                                const i = headerPTR.get(column);
                                const maxDistance = files[selectedFile]?.mapping?.[column]?.maxDistance ?? 0;
                                if (maxDistance === 0 || [...segmenter.segment(term)].length > 64) {
                                    return row[i]?.toLowerCase().includes(term.toLowerCase());
                                } else {
                                    return bitap(row[i]?.toLowerCase(), term.toLowerCase(), maxDistance) !== -1;
                                }
                            });
                        });
                    }).map((row) => {
                        const pkeyIndex = headerPTR.get(files[selectedFile]?.pkey) ?? 0;
                        const id = encodeURIComponent(row[pkeyIndex]);
                        return h("tr", {
                            onClick: () => setHash(id),
                            id,
                            class: id === hash ? "bg-orange-50" : "cursor-pointer hover:bg-orange-50"
                        }, storedColumns?.map((column) => {
                            const i = headerPTR.get(column);
                            return h("td", {class: "text-lg text-orange-600 whitespace-pre-line break-all px-6 py-4"}, row[i] ? markdownLinkify(row[i]) : "");
                        }));
                    })
                })()),
            ]),
        ])
    );
}

export default App;
