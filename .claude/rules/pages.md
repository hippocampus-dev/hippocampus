---
paths:
  - "pages/**"
---

* Use symlinks with relative paths to expose files from other directories
* When adding symlinks, also update the symlink list in `terraform/cloudflare.tf` (Cloudflare workaround)
* API endpoints (`/api/*`) require GitHub Actions OIDC token authentication

## Static File Serving

| Scenario | Method |
|----------|--------|
| Pages-specific content | Create file directly in `pages/` |
| Expose existing file | Create symlink to original |
