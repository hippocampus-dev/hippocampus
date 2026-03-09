export const stringifyJSON = (rows: any[][], path?: string): string => {
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
            try {
                const json = JSON.parse(value);
                if (typeof json === "object" && json !== null) {
                    v[key] = json;
                } else {
                    v[key] = value;
                }
            } catch (_) {
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
        return JSON.stringify(o);
    }

    return JSON.stringify(data);
}
