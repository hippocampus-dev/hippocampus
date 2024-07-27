import {parse, parseDocument, stringify, Node, Document, YAMLMap, YAMLSeq, Scalar} from "yaml";

export const parseYAML = (yaml: string, path?: string): any[][] => {
    let data = parse(yaml);

    if (path) {
        const keys = path.split(".");
        keys.forEach((key) => {
            if (typeof data === "object" && !Array.isArray(data)) {
                data = data[key];
            }
        });
    }

    if (!Array.isArray(data)) {
        throw new Error("Invalid YAML");
    }

    const extractKeys = (o: object, prefix = ""): string[] => {
        const keys = [];
        Object.entries(o).forEach(([k, v]) => {
            const newKey = prefix ? `${prefix}.${k}` : k;
            if (v && typeof v === "object" && !Array.isArray(v)) {
                keys.push(...extractKeys(v, newKey));
            } else {
                keys.push(newKey);
            }
        });
        return keys;
    };

    const headersSet: Set<string> = new Set();
    data.forEach((o: object) => {
        extractKeys(o).forEach((key) => {
            headersSet.add(key);
        });
    });
    const headers = Array.from(headersSet);

    const rows: any[][] = [headers];

    data.forEach((o) => {
        const row = headers.map((header) => {
            const keys = header.split(".");
            let v = o;
            for (const key of keys) {
                if (v[key]) {
                    v = v[key];
                } else {
                    return "";
                }
            }
            return Array.isArray(v) ? stringify(v) : v;
        });
        rows.push(row);
    });

    return rows;
}

export const stringifyYAML = (rows: any[][], path?: string): string => {
    const headers = rows[0];

    const data = rows.slice(1).map((row) => {
        const o: any = {};
        headers.forEach((header, i) => {
            let v = o;
            const keys = header.split(".");
            keys.slice(0, -1).forEach((key: string) => {
                if (v[key]) {
                    v = v[key];
                } else {
                    v[key] = {};
                    v = v[key];
                }
            });

            const value = row[i];
            if (value === "") {
                return;
            }

            const key = keys[keys.length - 1];
            const yaml = parse(value);
            if (typeof yaml === "object" && yaml !== null) {
                v[key] = yaml;
            } else {
                v[key] = value;
            }
        });
        return o;
    });

    if (path) {
        const keys = path.split(".");
        const o: any = {};
        let v = o;
        keys.slice(0, -1).forEach((key) => {
            v[key] = {};
            v = v[key];
        });
        v[keys[keys.length - 1]] = data;
        return stringify(o);
    }

    return stringify(data);
}

export const parseYAMLDocument = (yaml: string, path?: string): any[][][] => {
    const document = parseDocument(yaml);
    let data = document.contents;

    if (path) {
        const keys = path.split(".");
        keys.forEach((key) => {
            if (data instanceof YAMLMap) {
                data = data.get(key);
            }
        });
    }

    if (!(data instanceof YAMLSeq)) {
        throw new Error("Invalid YAML");
    }

    const extractKeys = (o: Node, prefix = ""): string[] => {
        const keys = [];
        if (o instanceof YAMLMap) {
            o.items.forEach((item) => {
                const key = item.key?.toString();
                if (key) {
                    const newKey = prefix ? `${prefix}.${key}` : key;
                    if (item.value && item.value instanceof YAMLMap) {
                        keys.push(...extractKeys(item.value, newKey));
                    } else {
                        keys.push(newKey);
                    }
                }
            });
        }
        return keys;
    }

    const headersSet: Set<string> = new Set();
    data.items.forEach((item) => {
        extractKeys(item).forEach((key) => {
            headersSet.add(key);
        });
    });
    const headers = Array.from(headersSet);

    const commentHeader = ["_comment", document.commentBefore || undefined]
    const rows: any[][][] = [[commentHeader, ...headers.map((header) => [header, undefined])]];

    data.items.forEach((item) => {
        const commentCell = ["", item.commentBefore];

        const row = headers.map((header, i) => {
            const keys = header.split(".");
            let v: any = item;
            for (const key of keys) {
                if (v instanceof YAMLMap) {
                    v = v.get(key, true);
                } else {
                    return ["", undefined];
                }
            }
            if (v instanceof YAMLSeq) {
                return [stringify(v.items), undefined];
            }
            if (v instanceof Scalar) {
                return [v.value, v.comment];
            }
            return ["", undefined];
        });
        rows.push([commentCell, ...row]);
    });

    return rows;
}

export const stringifyYAMLDocument = (rows: any[][][], path?: string): string => {
    let _commentHeader = rows[0][0];
    const headers = rows[0].slice(1);

    const data = new YAMLSeq();
    rows.slice(1).forEach((row) => {
        let o: YAMLMap = new YAMLMap();
        const commentCell = row[0];
        o.commentBefore = commentCell[1];
        headers.forEach(([header, _], i) => {
            let v: any = o;
            const keys = header.split(".");
            keys.slice(0, -1).forEach((key: string) => {
                const next = v.get(key, true);
                if (next) {
                    v = next;
                } else {
                    v.set(key, new YAMLMap());
                    v = v.get(key, true);
                }
            });

            const value = row[i + 1][0];
            if (value === "") {
                return;
            }

            const key = keys[keys.length - 1];
            const document = parseDocument(value);
            if (document.contents instanceof YAMLMap || document.contents instanceof YAMLSeq) {
                v.set(key, document);
            } else {
                const scalar = new Scalar(value);
                scalar.comment = row[i + 1][1];
                v.set(key, scalar);
            }
        });
        data.add(o);
    });

    if (path) {
        const keys = path.split(".");
        let o: YAMLMap = new YAMLMap();
        let v: any = o;
        keys.slice(0, -1).forEach((key: string) => {
            v.set(key, new YAMLMap());
            v = v.get(key, true);
        });
        v.set(keys[keys.length - 1], data);
        return new Document(o).toString();
    }

    return new Document(data).toString();
}
