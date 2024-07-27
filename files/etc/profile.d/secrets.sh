function e() {
  local n=$1
  local f="/home/kai/.secrets/$n"
  if [ -e $f ]; then
    echo "export $n=$(cat $f)"
  fi
}

eval "$(e GITHUB_TOKEN)"
eval "$(e OPENAI_API_KEY)"
eval "$(e OPENAI_SESSION_TOKEN)"
eval "$(e SLACK_APP_TOKEN)"
eval "$(e SLACK_BOT_TOKEN)"
eval "$(e SLACK_SIGNING_SECRET)"
eval "$(e SLACK_AUTOMATION_GOOGLE_CLIENT_ID)"
eval "$(e SLACK_AUTOMATION_GITHUB_CLIENT_ID)"
eval "$(e SLACK_AUTOMATION_SLACK_CLIENT_ID)"
eval "$(e GOOGLE_PROJECT_ID)"
eval "$(e GOOGLE_CLIENT_ID)"
eval "$(e GOOGLE_CLIENT_SECRET)"
eval "$(e GOOGLE_PRE_ISSUED_REFRESH_TOKEN)"
eval "$(e TAURI_SIGNING_PRIVATE_KEY)"
eval "$(e TAURI_SIGNING_PRIVATE_KEY_PASSWORD)"
