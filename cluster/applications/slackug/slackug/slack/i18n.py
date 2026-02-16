import enum


class Locale(enum.StrEnum):
    English = "English"
    Japanese = "Japanese"
    Vietnamese = "Vietnamese"


TRANSLATION_TABLE = {
    "A temporary error has occurred.\nPlease try again.": {
        Locale.Japanese: "一時的なエラーが発生しました。再度お試しください。",
        Locale.Vietnamese: "Đã xảy ra lỗi tạm thời.\nVui lòng thử lại.",
    },
    "An unknown error has occurred": {
        Locale.Japanese: "不明なエラーが発生しました",
        Locale.Vietnamese: "Đã xảy ra lỗi không xác định",
    },
    "You are not available.": {
        Locale.Japanese: "あなたは使用できません。",
        Locale.Vietnamese: "Bạn không có sẵn.",
    },
    "Not available on this channel.": {
        Locale.Japanese: "このチャンネルでは使用できません。",
        Locale.Vietnamese: "Không có sẵn trên kênh này.",
    },
}


def translate(text, locale=Locale.English) -> str:
    return TRANSLATION_TABLE.get(text, {}).get(locale, text)
