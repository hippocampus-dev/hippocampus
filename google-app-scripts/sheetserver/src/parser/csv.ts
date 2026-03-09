export const stringifyRFC4180 = (rows: string[][]): string => {
    const needsDoubleQuote = (field: string): boolean => {
        return field.includes(",") || field.includes('""') || field.includes("\r\n") || field.includes("\r") || field.includes("\n");
    }

    let csv = "";
    rows.forEach((row) => {
        const empties = row.filter((value) => value === "");
        if (empties.length !== row.length) {
            row.forEach((field, i) => {
                if (typeof field === "string") {
                    field = field.replace(/"/g, '""');
                    field = needsDoubleQuote(field) ? `"${field}"` : field;
                }
                csv += field;

                if (i !== row.length - 1) {
                    csv += ",";
                }
            });
        }
        csv += "\n";
    });
    return csv;
};
