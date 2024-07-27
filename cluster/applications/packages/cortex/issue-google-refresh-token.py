import google_auth_oauthlib.flow
import pydantic

import cortex


class Settings(pydantic.BaseSettings):
    google_project_id: str
    google_client_id: str
    google_client_secret: str


def main():
    s = Settings()

    credentials = google_auth_oauthlib.flow.InstalledAppFlow.from_client_config(
        {
            "installed": {
                "project_id": s.google_project_id,
                "client_id": s.google_client_id,
                "client_secret": s.google_client_secret,
                "auth_uri": "https://accounts.google.com/o/oauth2/auth",
                "token_uri": "https://accounts.google.com/o/oauth2/token",
                "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
                "redirect_uris": [
                    "http://127.0.0.1"
                ],
            }
        },
        cortex.GOOGLE_OAUTH_SCOPES,
    ).run_local_server(port=0)

    print(credentials.refresh_token)


if __name__ == "__main__":
    main()
