const __ = (f: Function) => {
    try {
        return f();
    } catch (e) {
        const email = PropertiesService.getScriptProperties().getProperty("notification")!;
        GmailApp.sendEmail(email, "Sheethub Error", e.message);
        throw e;
    }
}

const doGet = (e: GoogleAppsScript.Events.DoGet) => {
    return __(() => {
        // @ts-ignore
        return _.doGet(e);
    });
}

const setProperties = () => {
    PropertiesService.getScriptProperties().setProperties({}, false);
}
