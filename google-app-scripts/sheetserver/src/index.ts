import { stringifyRFC4180 } from "./parser/csv";
import { stringifyJSON } from "./parser/json";
import { stringifyYAML } from "./parser/yaml";

export const doGet = (e: GoogleAppsScript.Events.DoGet) => {
    const spreadsheetId = e.parameter["spreadsheetId"];
    const gid = e.parameter["gid"];
    if (!spreadsheetId || !gid) {
        return ContentService.createTextOutput(JSON.stringify({
            error: "spreadsheetId and gid are required",
        })).setMimeType(ContentService.MimeType.JSON);
    }
    const format = e.parameter["format"];

    const spreadsheet = Sheets.Spreadsheets.get(spreadsheetId);
    const sheet = spreadsheet.sheets.find((sheet) => sheet.properties.sheetId === Number(gid));
    if (!sheet) {
        return ContentService.createTextOutput(JSON.stringify({
            error: "gid is invalid",
        })).setMimeType(ContentService.MimeType.JSON);
    }
    const rows = Sheets.Spreadsheets.Values.get(spreadsheetId, sheet.properties.title).values;
    switch (format) {
        case "json":
            return ContentService.createTextOutput(stringifyJSON(rows)).setMimeType(ContentService.MimeType.TEXT);
        case "yaml":
            return ContentService.createTextOutput(stringifyYAML(rows)).setMimeType(ContentService.MimeType.TEXT);
        default:
            return ContentService.createTextOutput(stringifyRFC4180(rows)).setMimeType(ContentService.MimeType.TEXT);
    }
}
