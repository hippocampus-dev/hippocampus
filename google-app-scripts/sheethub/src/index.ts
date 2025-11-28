import {GitHubBlob, GitHubCommit, GitHubFile, GitHubRef, GitHubTrees} from "./types";
import {parseRFC4180, stringifyRFC4180} from "./parser/csv";
import {parseYAML, stringifyYAML} from "./parser/yaml";
import {RailsMessageEncryptor} from "./rails_message_encryptor";

class GitClient {
    private readonly repository: string;
    private readonly token: string;
    private staging: { path: string, mode: string, sha: string }[];

    constructor(repository: string, token: string) {
        this.repository = repository;
        this.token = token;

        this.staging = [];
    }

    checkout(branch: string): GitHubRef {
        const result = UrlFetchApp.fetch(`https://api.github.com/repos/${this.repository}/git/refs/heads/${branch}`, {
            method: "get",
            headers: {
                "Accept": "application/vnd.github+json",
                "Authorization": `Bearer ${this.token}`,
                "X-GitHub-Api-Version": "2022-11-28",
            },
        }).getContentText();
        return JSON.parse(result);
    }

    add(path: string, content: string, mode: string = "100644") {
        const result = UrlFetchApp.fetch(`https://api.github.com/repos/${this.repository}/git/blobs`, {
            method: "post",
            headers: {
                "Accept": "application/vnd.github+json",
                "Authorization": `Bearer ${this.token}`,
                "X-GitHub-Api-Version": "2022-11-28",
            },
            payload: JSON.stringify({
                content: Utilities.base64Encode(content, Utilities.Charset.UTF_8),
                encoding: "base64",
            }),
        }).getContentText();
        const blob: GitHubBlob = JSON.parse(result);
        this.staging.push({path, mode, sha: blob.sha});
    }

    commit(message: string, sha: string): GitHubCommit {
        const tree = this.staging.map((file) => {
            return {
                path: file.path,
                mode: file.mode,
                type: "blob",
                sha: file.sha,
            }
        });
        const treesResult = UrlFetchApp.fetch(`https://api.github.com/repos/${this.repository}/git/trees`, {
            method: "post",
            headers: {
                "Accept": "application/vnd.github+json",
                "Authorization": `Bearer ${this.token}`,
                "X-GitHub-Api-Version": "2022-11-28",
            },
            payload: JSON.stringify({
                base_tree: sha,
                tree,
            }),
        }).getContentText();
        const trees: GitHubTrees = JSON.parse(treesResult);

        const commitResult = UrlFetchApp.fetch(`https://api.github.com/repos/${this.repository}/git/commits`, {
            method: "post",
            headers: {
                "Accept": "application/vnd.github+json",
                "Authorization": `Bearer ${this.token}`,
                "X-GitHub-Api-Version": "2022-11-28",
            },
            payload: JSON.stringify({
                message,
                tree: trees.sha,
                parents: [sha],
            }),
        }).getContentText();
        return JSON.parse(commitResult);
    }

    push(branch: string, sha: string): GitHubRef {
        const response = UrlFetchApp.fetch(`https://api.github.com/repos/${this.repository}/git/refs/heads/${branch}`, {
            muteHttpExceptions: true,
            method: "get",
            headers: {
                "Accept": "application/vnd.github+json",
                "Authorization": `Bearer ${this.token}`,
                "X-GitHub-Api-Version": "2022-11-28",
            },
        });

        if (response.getResponseCode() === 200) {
            const result = UrlFetchApp.fetch(`https://api.github.com/repos/${this.repository}/git/refs/heads/${branch}`, {
                method: "patch",
                headers: {
                    "Accept": "application/vnd.github+json",
                    "Authorization": `Bearer ${this.token}`,
                    "X-GitHub-Api-Version": "2022-11-28",
                },
                payload: JSON.stringify({
                    sha: sha,
                }),
            }).getContentText();
            return JSON.parse(result);
        } else {
            const result = UrlFetchApp.fetch(`https://api.github.com/repos/${this.repository}/git/refs`, {
                method: "post",
                headers: {
                    "Accept": "application/vnd.github+json",
                    "Authorization": `Bearer ${this.token}`,
                    "X-GitHub-Api-Version": "2022-11-28",
                },
                payload: JSON.stringify({
                    ref: `refs/heads/${branch}`,
                    sha: sha,
                }),
            }).getContentText();
            return JSON.parse(result);
        }
    }
}

class GitHubClient {
    private readonly repository: string;
    private readonly token: string;

    constructor(repository: string, token: string) {
        this.repository = repository;
        this.token = token;
    }

    createPullRequest(title: string, body: string, head: string, base: string) {
        UrlFetchApp.fetch(`https://api.github.com/repos/${this.repository}/pulls`, {
            method: "post",
            headers: {
                "Accept": "application/vnd.github+json",
                "Authorization": `Bearer ${this.token}`,
                "X-GitHub-Api-Version": "2022-11-28",
            },
            payload: JSON.stringify({
                title,
                body,
                head,
                base,
            }),
        });
    }
}

enum ModificationType {
    Insert = "insert",
    Edit = "edit",
    Delete = "delete",
}

interface History {
    type: ModificationType;
    email: string;
}

interface RowHistory {
    [row: number]: History[]
}

const render = (sheet: GoogleAppsScript.Spreadsheet.Sheet, rows: string[][]) => {
    let maxColumns = 0;
    let rowIndex = 0;
    let cursor = 1;

    const documentProperties = PropertiesService.getDocumentProperties();
    const property = documentProperties.getProperty(sheet.getName());
    const rowHistory: RowHistory = property === null ? {} : JSON.parse(property);

    while (rowIndex < rows.length) {
        maxColumns = Math.max(maxColumns, rows[rowIndex].length);

        const targetRange = sheet.getRange(cursor, 1, 1, rows[rowIndex].length);

        const histories = rowHistory[rowIndex + 1];
        if (histories !== undefined) {
            const {type} = histories[histories.length - 1];
            switch (type) {
                case ModificationType.Insert:
                    cursor++;
                    break;
                case ModificationType.Edit:
                    cursor++;
                    rowIndex++;
                    break;
                case ModificationType.Delete:
                    rowIndex++;
                    break;
            }
            continue;
        }

        targetRange.setValues([rows[rowIndex]]);
        targetRange.setVerticalAlignment("top");
        targetRange.setWrap(true);

        cursor++;
        rowIndex++;
    }

    const remainingHistory = Object.keys(rowHistory).filter((row) => {
        return parseInt(row) >= cursor;
    });
    cursor += remainingHistory.length;

    sheet.setFrozenRows(1);
    if (maxColumns > sheet.getMaxColumns()) {
        sheet.insertColumnsAfter(sheet.getMaxColumns(), maxColumns - sheet.getMaxColumns());
    }
    sheet.getRange(1, maxColumns + 1, sheet.getMaxRows(), sheet.getMaxColumns() - maxColumns).clearContent();
    sheet.getRange(cursor, 1, sheet.getMaxRows() - cursor, sheet.getMaxColumns()).clearContent();
}

export const customOnOpen = () => {
    const repository = PropertiesService.getScriptProperties().getProperty("repository")!;
    const token = PropertiesService.getScriptProperties().getProperty("token")!;

    const masterKey = PropertiesService.getScriptProperties().getProperty("master.key");

    const spreadsheet = SpreadsheetApp.getActiveSpreadsheet();
    const sheets = spreadsheet.getSheets();
    sheets.forEach((sheet) => {
        const name = sheet.getName();
        const [filename, anchor] = name.split("#");

        const response = UrlFetchApp.fetch(`https://api.github.com/repos/${repository}/contents/${filename}`, {
            method: "get",
            headers: {
                "Accept": "application/vnd.github+json",
                "Authorization": `Bearer ${token}`,
                "X-GitHub-Api-Version": "2022-11-28",
            },
            muteHttpExceptions: true,
        });
        if (response.getResponseCode() !== 200) {
            console.error(response.getContentText());
            return;
        }
        const result = response.getContentText();
        const file: GitHubFile = JSON.parse(result);
        const content = Utilities.newBlob(Utilities.base64Decode(file.content)).getDataAsString();

        const parts = filename.split(".");

        let extension = parts.pop();
        let filter = (content: string): string => content;
        if (extension === "enc") {
            extension = parts.pop();

            const messageEncryptor = new RailsMessageEncryptor(masterKey!);
            messageEncryptor.addEntropy(256);

            filter = (content: string): string => {
                return messageEncryptor.decrypt(content);
            }
        }
        switch (extension) {
            case "csv":
                render(sheet, parseRFC4180(filter(content)));
                break;
            case "yml":
            case "yaml":
                render(sheet, parseYAML(filter(content), anchor));
                break;
        }
    });
}

const saveHistory = (sheet: GoogleAppsScript.Spreadsheet.Sheet, i: number, history: History) => {
    const documentProperties = PropertiesService.getDocumentProperties();
    const property = documentProperties.getProperty(sheet.getName());
    if (property === null) {
        documentProperties.setProperty(sheet.getName(), JSON.stringify({[i]: [history]}));
        return;
    }

    const rowHistory: RowHistory = JSON.parse(property);
    // Detect consecutive insert or delete.
    while (i <= sheet.getMaxRows()) {
        const histories = rowHistory[i];
        if (histories === undefined) {
            break;
        }
        const {type} = histories[histories.length - 1];
        if (type === ModificationType.Insert || type === ModificationType.Delete) {
            i++;
        } else {
            break;
        }
    }

    if (rowHistory[i] === undefined) {
        rowHistory[i] = [history];
    } else {
        rowHistory[i].push(history);
    }
    documentProperties.setProperty(sheet.getName(), JSON.stringify(rowHistory));
}

export const customOnEdit = (e: GoogleAppsScript.Events.SheetsOnEdit) => {
    const history = {
        type: ModificationType.Edit,
        email: Session.getActiveUser().getEmail(),
    };

    for (let i = e.range.getRow(); i <= e.range.getLastRow(); i++) {
        saveHistory(e.source.getActiveSheet(), i, history);
    }
}

export const customOnChange = (e: GoogleAppsScript.Events.SheetsOnChange) => {
    switch (e.changeType) {
        case "INSERT_ROW":
            saveHistory(e.source.getActiveSheet(), e.source.getActiveRange().getRowIndex(), {
                type: ModificationType.Insert,
                email: Session.getActiveUser().getEmail(),
            });
            break;
        case "REMOVE_ROW":
            saveHistory(e.source.getActiveSheet(), e.source.getActiveRange().getRowIndex(), {
                type: ModificationType.Delete,
                email: Session.getActiveUser().getEmail(),
            });
            break;
    }
}

export const sync = () => {
    const documentProperties = PropertiesService.getDocumentProperties();
    const keys = documentProperties.getKeys();

    // No changes.
    if (keys.length === 0) {
        return;
    }

    const spreadsheet = SpreadsheetApp.getActiveSpreadsheet();
    const sheets = spreadsheet.getSheets();

    const repository = PropertiesService.getScriptProperties().getProperty("repository")!;
    const token = PropertiesService.getScriptProperties().getProperty("token")!;
    const gitClient = new GitClient(repository, token);
    const gitHubClient = new GitHubClient(repository, token);

    const masterKey = PropertiesService.getScriptProperties().getProperty("master.key");

    const base = PropertiesService.getScriptProperties().getProperty("base")!;
    const baseRef = gitClient.checkout(base);
    sheets.forEach((sheet) => {
        const name = sheet.getName();
        if (!keys.includes(name)) {
            return;
        }
        const rows = sheet.getDataRange().getValues();

        const [filename, anchor] = name.split("#");
        const parts = filename.split(".");

        let extension = parts.pop();
        let filter = (content: string): string => content;
        if (extension === "enc") {
            extension = parts.pop();

            const messageEncryptor = new RailsMessageEncryptor(masterKey!);
            messageEncryptor.addEntropy(256);

            filter = (content: string): string => {
                return messageEncryptor.encrypt(content);
            };
        }
        switch (extension) {
            case "csv":
                gitClient.add(name, filter(stringifyRFC4180(rows)));
                break;
            case "yml":
            case "yaml":
                gitClient.add(filename, filter(stringifyYAML(rows, anchor)));
                break;
        }
    });
    const commit = gitClient.commit("Update sheets", baseRef.object.sha);
    const ref = gitClient.push(new Date().getTime().toString(), commit.sha);

    let body = "";

    keys.forEach((sheet) => {
        const rowHistory: RowHistory = JSON.parse(documentProperties.getProperty(sheet));
        Object.keys(rowHistory).forEach((row) => {
            const modifications = new Set();
            rowHistory[row].forEach((history: History) => {
                modifications.add(`${history.email}(${history.type})`);
            });
            body += `- https://github.com/${repository}/blob/${ref.object.sha}/${sheet}#L${row}\n`;
            modifications.forEach((modification) => {
                body += `  - ${modification}\n`;
            });
        });
    });

    gitHubClient.createPullRequest("Update sheets", body, ref.ref, base);

    documentProperties.deleteAllProperties();
}
