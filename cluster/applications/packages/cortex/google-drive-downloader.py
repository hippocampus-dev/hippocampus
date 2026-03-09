import argparse
import io
import os

import google_auth_httplib2
import google_auth_oauthlib.flow
import googleapiclient.discovery
import googleapiclient.errors
import googleapiclient.http
import httplib2
import pydantic_settings

import cortex


class Settings(pydantic_settings.BaseSettings):
    model_config = pydantic_settings.SettingsConfigDict(extra="allow", env_file=".env")

    google_project_id: str
    google_client_id: str
    google_client_secret: str


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("urls", nargs="+")
    args = parser.parse_args()

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

    mime_types = {
        "document": "text/plain",
        "spreadsheets": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        "presentation": "application/pdf",
    }

    extensions = {
        "document": "txt",
        "spreadsheets": "xlsx",
        "presentation": "pdf",
    }

    http = google_auth_httplib2.AuthorizedHttp(
        credentials,
        http=httplib2.Http(ca_certs=os.getenv("HTTPLIB2_CA_CERTS")),
    )
    drive = googleapiclient.discovery.build("drive", "v3", http=http)

    for url in args.urls:
        shards = url.split("/")
        file_id, file_type = shards[-2], shards[-4]

        mime_type = mime_types.get(file_type)
        if mime_type is None:
            continue

        file = drive.files().get(fileId=file_id, supportsAllDrives=True).execute()
        title = file["name"]

        buffer = io.BytesIO()

        try:
            buffer = io.BytesIO()
            document = drive.files().export_media(fileId=file_id, mimeType=mime_type)
            downloader = googleapiclient.http.MediaIoBaseDownload(buffer, document)

            done = False
            while done is False:
                status, done = downloader.next_chunk()

            with open(f"{title}.{extensions[file_type]}", "wb") as f:
                f.write(buffer.getvalue())
        except googleapiclient.errors.HttpError:
            buffer = io.BytesIO()
            # `export_media` is not supported for this file type
            document = drive.files().get_media(fileId=file_id)
            downloader = googleapiclient.http.MediaIoBaseDownload(buffer, document)

            done = False
            while done is False:
                status, done = downloader.next_chunk()

            with open(title, "wb") as f:
                f.write(buffer.getvalue())


if __name__ == "__main__":
    main()
