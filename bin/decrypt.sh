#!/usr/bin/env -S bash -l

set -eo pipefail
trap 'echo "exit $?: $BASH_COMMAND(line $LINENO)" >&2' ERR

ENTRYPOINT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

while IFS= read -r file; do
  armyknife rails credentials:show "$file" > "${file%.enc}"
done < <(find "${ENTRYPOINT}/.." -type f -name '*.enc')
