// https://datatracker.ietf.org/doc/html/rfc4180#section-2
// If fields are not enclosed with double quotes, then double quotes may not appear inside the fields.
export class RFC4180DoubleQuoteError extends Error {
    constructor() {
        super("the field containing double quotes must be enclosed in double quotes");
        this.name = "RFC4180DoubleQuoteError";
    }
}

// https://datatracker.ietf.org/doc/html/rfc4180
export const parseRFC4180 = (csv: string): string[][] => {
    const rows: string[][] = []

    const newLineRegexp = /(\r\n|\r)/g;
    const newCSV = csv.replace(newLineRegexp, "\n");
    const lineEndCharacter = "\n";

    let row: string[] = [];
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
}

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
}
