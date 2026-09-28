---
paths:
  - ".github/scripts/cleanup.sh"
---

* Never delete `/usr/lib/python3` - apt's `APT::Update::Post-Invoke-Success` hook imports `apt_pkg` from there through `/usr/lib/cnf-update-db`, so a job calling this script before `apt-get update` gets exit 100 from the hook's failure even though the index fetch itself succeeded, with nothing in apt's output naming the deletion
