const __ = (f: Function) => {
    try {
        return f();
    } catch (e) {
        const email = PropertiesService.getScriptProperties().getProperty("notification")!;
        GmailApp.sendEmail(email, "Sheethub Error", e.message);
        throw e;
    }
}

const customOnOpen = () => {
    return __(() => {
        // @ts-ignore
        return _.customOnOpen();
    });
}

const customOnEdit = (e: GoogleAppsScript.Events.SheetsOnEdit) => {
    return __(() => {
        // @ts-ignore
        return _.customOnEdit(e);
    });
}

const customOnChange = (e: GoogleAppsScript.Events.SheetsOnChange) => {
    return __(() => {
        // @ts-ignore
        return _.customOnChange(e);
    });
}

const sync = () => {
    return __(() => {
        // @ts-ignore
        return _.sync();
    });
}

const setProperties = (repository: string, base: string, token: string, notification: string) => {
    PropertiesService.getScriptProperties().setProperties({
        repository: repository,
        base: base,
        token: token,
        notification: notification,
    }, false);
}

const updateTrigger = () => {
    const spreadsheet = SpreadsheetApp.getActiveSpreadsheet();
    ScriptApp.getProjectTriggers().forEach((trigger) => {
        ScriptApp.deleteTrigger(trigger);
    });
    ScriptApp.newTrigger("customOnOpen").forSpreadsheet(spreadsheet).onOpen().create();
    ScriptApp.newTrigger("customOnEdit").forSpreadsheet(spreadsheet).onEdit().create();
    ScriptApp.newTrigger("customOnChange").forSpreadsheet(spreadsheet).onChange().create();
    ScriptApp.newTrigger("sync").timeBased().everyDays(1).atHour(0).create();
}
