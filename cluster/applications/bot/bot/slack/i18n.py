import enum


class Locale(enum.StrEnum):
    English = "English"
    Japanese = "Japanese"


TRANSLATION_TABLE = {
    "The following task is being executed:": {
        Locale.Japanese: "以下のタスクを実行しています:",
    },
    "You have reached {rate_limit_interval_seconds}-second usage limit. Please wait a moment and try again.": {
        Locale.Japanese: "{rate_limit_interval_seconds}秒間の利用量の限界に達しました。しばらく待ってからもう一度お試しください。",
    },
    "You have reached your OpenAI API billing limit.\nPlease wait a moment and try again.": {
        Locale.Japanese: "OpenAI APIの利用量の限界に達しました。\nしばらく待ってからもう一度お試しください。",
    },
    "The maximum number of conversations has been exceeded.\nPlease create a new thread": {
        Locale.Japanese: "やり取りの上限を超えました。\n新しいスレッドを作成してください。",
    },
    "Your message contains violent or explicit.": {
        Locale.Japanese: "あなたのメッセージには暴力的または露骨な表現が含まれています。",
    },
    "Could not resolve complex task.": {
        Locale.Japanese: "複雑なタスクを解決できませんでした。",
    },
    "The price for this response was {price}USD.": {
        Locale.Japanese: "この回答には{price}USDの料金がかかりました。",
    },
    ":warning: The conversation are very long.\nIt is recommend to create a new thread due to the lower accuracy.": {
        Locale.Japanese: ":warning: やり取りが非常に長くなっています。精度が低くなるため新しいスレッドを作成することをお勧めします。",
    },
    "A temporary error occurred.\nPlease try again.": {
        Locale.Japanese: "一時的なエラーが発生しました。再度お試しください。",
    },
    "An unknown error has occurred": {
        Locale.Japanese: "不明なエラーが発生しました",
    },
    "You are not available.": {
        Locale.Japanese: "あなたは使用できません。",
    },
    "Not available on this channel.": {
        Locale.Japanese: "このチャンネルでは使用できません。",
    },
    "Click `Report' to send conversations in this thread to the development team.": {
        Locale.Japanese: "`報告する` をクリックするとこのスレッドの会話を開発チームに送信できます。",
    },
    "Do you want to report it?": {
        Locale.Japanese: "報告しますか？",
    },
    "Are you sure you want this thread conversations sent?": {
        Locale.Japanese: "このスレッドのやり取りを送信してもよろしいですか？",
    },
    "Report": {
        Locale.Japanese: "報告する",
    },
    "Click `Delete` to delete the previous answer.": {
        Locale.Japanese: "`削除する` をクリックすると直前の回答を削除することができます。",
    },
    "Do you want to delete it?": {
        Locale.Japanese: "削除しますか？",
    },
    "Are you sure you want to delete the previous answer?": {
        Locale.Japanese: "直前の回答が削除されますがよろしいですか？",
    },
    "Delete": {
        Locale.Japanese: "削除する",
    },
    "Cancel": {
        Locale.Japanese: "キャンセル",
    },
}


def translate(text, locale=Locale.English) -> str:
    return TRANSLATION_TABLE.get(text, {}).get(locale, text)
