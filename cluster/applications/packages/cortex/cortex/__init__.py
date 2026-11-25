import requests

import cortex.exceptions

GOOGLE_OAUTH_SCOPES = [
    "https://www.googleapis.com/auth/drive.readonly",
]


class URLShortener:
    url_shortener_url: str

    def __init__(self, url_shortener_url: str):
        self.url_shortener_url = url_shortener_url

    def shorten(self, url: str) -> str:
        try:
            response = requests.post(self.url_shortener_url, data=url, timeout=30)
            if response.status_code != 200:
                e = ValueError(f"invalid response status: {response.status_code}")
                match response.status_code:
                    case 409 | 429 | 502 | 503 | 504:
                        raise cortex.exceptions.RetryableError(e) from e
                raise e
            return f"{self.url_shortener_url}/{response.text}"
        except (
            requests.exceptions.ConnectionError,  # ECONNREFUSED, EPIPE, ECONNRESET
            requests.exceptions.Timeout,
        ) as e:
            raise cortex.exceptions.RetryableError(e) from e
