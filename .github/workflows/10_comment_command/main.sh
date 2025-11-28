#!/usr/bin/env bash

set -e

function cortex() {
  ID_TOKEN=$(curl -fsSL -H "Authorization: Bearer ${ACTIONS_ID_TOKEN_REQUEST_TOKEN}" "${ACTIONS_ID_TOKEN_REQUEST_URL}&audience=${GITHUB_ACTION_PATH}" | jq -r .value)
  echo "::add-mask::$ID_TOKEN"
  response=$(curl -fsSL -X POST -H "Content-Type: application/json" -H "Authorization: Bearer ${ID_TOKEN}" http://cortex-api.cortex-api.svc.cluster.local:8080/v1/chat/completions -d "$(echo ${@} | jq -sc --arg model ${MODEL} '. as $messages | {model: $model, messages: $messages}')")

  if [ "$(echo ${response} | jq -r .error)" != "null" ]; then
    echo ${response} | jq -r .error 1>&2
    exit 1
  fi

  echo ${response} | jq -r .choices[0].message.content
}

if [ "${COMMENT_BODY}" == "/rebase" ] && [ "${PULL_REQUEST_EVENT}" != "" ]; then
  pr=$(curl -fsSL -H "Authorization: Bearer ${GITHUB_TOKEN}" https://api.github.com/repos/${GITHUB_REPOSITORY}/pulls/${ISSUE_NUMBER})
  mergeableState=$(echo ${pr} | jq -r .mergeable_state)
  base=$(echo ${pr} | jq -r .base.ref)
  head=$(echo ${pr} | jq -r .head.ref)
  if [ "${mergeableState}" != "dirty" ]; then
    exit 0
  fi

  git config --global user.name "kaidotio"
  git config --global user.email "kaidotio@gmail.com"
  git fetch origin ${head}
  git checkout -b ${head} origin/${head}
  git fetch origin ${base}
  if git rebase origin/${base}; then
    git push -f origin ${head}
  else
    curl -fsSL -X POST -H "Authorization: Bearer ${GITHUB_TOKEN}" https://api.github.com/repos/${GITHUB_REPOSITORY}/issues/${ISSUE_NUMBER}/comments -d "$(jq -n --arg body "${body}" '{body: $body}')"
  fi
fi

if [ "${COMMENT_BODY}" == "/cortex review" ] && [ "${PULL_REQUEST_EVENT}" != "" ]; then
  pr=$(curl -fsSL -H "Authorization: Bearer ${GITHUB_TOKEN}" https://api.github.com/repos/${GITHUB_REPOSITORY}/pulls/${ISSUE_NUMBER})

  export GITHUB_EVENT_PATH=$(mktemp)
  cat <<EOS > ${GITHUB_EVENT_PATH}
{
  "repository": ${REPOSITORY_EVENT},
  "pull_request": ${pr}
}
EOS

  base=$(echo ${pr} | jq -r .base.ref)
  head=$(echo ${pr} | jq -r .head.ref)
  git fetch origin ${base} ${head}
  diff=$(git diff origin/${base}...origin/${head})

  systemPrompt=$(cat <<"EOS"
Your task is to find the issues in the given `git diff` and provide the correct format.

## Issues

- Misspelling
- Security issues
- Performance issues

## Format

The output MUST be `%f:%l: %m` format.
```
%f: File name
%l: Line number
%m: Message
```

Line number MUST be the line number of the file that calculated from the diff.

### Example
`@@ -223,6 +223,6 @@` means the diff is starting from line 223 and the diff is 6 lines long.
So if the misspelling is on diff line 3(**Skip couting `-` lines**), the output must be `filename:225: misspelling: misspelled_word`.

Input:
```
diff --git a/cluster/applications/bot/bot/main.py b/cluster/applications/bot/bot/main.py
index e5ca6dc8..8e0edb52 100644
--- a/cluster/applications/bot/bot/main.py
+++ b/cluster/applications/bot/bot/main.py
@@ -223,6 +223,6 @@ async def get_rate_limiter() -> cortex.rate_limit.RateLimiter:
                 redis_client.execute_command = types.MethodType(new_execute_command, redis_client)
-                global_rate_limiter = cortex.rate_limit.RedisSlidingRateLimiter(
+                global_rate_limiter = cortex.rate_limit.RedisSlidingRateLimter(
             case _:
-                raise NotImplementedError
+                raise NotImplementedErro
     return global_rate_limiter
```

Output:
cluster/applications/bot/bot/main.py:225: misspelling: RedisSlidingRateLimter
cluster/applications/bot/bot/main.py:227: misspelling: NotImplementedErro
EOS
)

  userPrompt="${diff}"

  system=$(jq -n --arg role "system" --arg content "${systemPrompt}" '{role: $role, content: $content}')
  user=$(jq -n --arg role "user" --arg content "${userPrompt}" '{role: $role, content: $content}')
  body=$(cortex "${system}" "${user}")

  echo "${body}"

  curl -fsSL https://github.com/reviewdog/reviewdog/releases/download/v0.20.1/reviewdog_0.20.1_Linux_x86_64.tar.gz | tar xz reviewdog
  echo "${body}" | REVIEWDOG_GITHUB_API_TOKEN=${GITHUB_TOKEN} ./reviewdog -efm='%f:%l: %m' -reporter=github-pr-review -filter-mode=nofilter
fi

if [ "${COMMENT_BODY}" == "/cortex translate" ]; then
  issue=$(curl -fsSL -H "Authorization: Bearer ${GITHUB_TOKEN}" https://api.github.com/repos/${GITHUB_REPOSITORY}/issues/${ISSUE_NUMBER})

  systemPrompt=$(cat <<"EOS"
Translate the given text according to the following rules.

## Rules
- Translate English to Japanese
- Translate Japanese to English
EOS
)

  userPrompt=$(echo ${issue} | jq -r .body)

  system=$(jq -n --arg role "system" --arg content "${systemPrompt}" '{role: $role, content: $content}')
  user=$(jq -n --arg role "user" --arg content "${userPrompt}" '{role: $role, content: $content}')
  body=$(cortex "${system}" "${user}")

  curl -fsSL -X POST -H "Authorization: Bearer ${GITHUB_TOKEN}" https://api.github.com/repos/${GITHUB_REPOSITORY}/issues/${ISSUE_NUMBER}/comments -d "$(jq -n --arg body "${body}" '{body: $body}')"
fi

if [ "${COMMENT_BODY}" == "/cortex summary" ]; then
  comments=$(curl -fsSL -H "Authorization: Bearer ${GITHUB_TOKEN}" https://api.github.com/repos/${GITHUB_REPOSITORY}/issues/${ISSUE_NUMBER}/comments)

  systemPrompt=$(cat <<"EOS"
Your task is to create a concise running summary of actions and information results in the provided text while keeping the input language, focusing on key and potentially important information to remember.
EOS
)

  userPrompt=$(echo ${comments} | jq -r .[].body)

  system=$(jq -n --arg role "system" --arg content "${systemPrompt}" '{role: $role, content: $content}')
  user=$(jq -n --arg role "user" --arg content "${userPrompt}" '{role: $role, content: $content}')
  body=$(cortex "${system}" "${user}")

  curl -fsSL -X POST -H "Authorization: Bearer ${GITHUB_TOKEN}" https://api.github.com/repos/${GITHUB_REPOSITORY}/issues/${ISSUE_NUMBER}/comments -d "$(jq -n --arg body "${body}" '{body: $body}')"
fi
