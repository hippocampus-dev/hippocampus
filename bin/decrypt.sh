#!/usr/bin/env -S bash -l

set -e

ENTRYPOINT=$(cd $(dirname ${BASH_SOURCE[0]}); pwd)

find ${ENTRYPOINT}/.. -type f -name '*.enc' | while IFS= read -r file; do
  armyknife rails credentials:show $file > ${file%.enc}
done
