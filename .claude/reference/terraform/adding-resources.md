# Adding New Resources

How to verify Terraform resource schemas before adding new resources.

## When to Use

- Adding new Terraform resources to any provider module
- Updating existing resources with new attributes

## Process

1. Check `versions.tf` for version constraints (e.g., `version = "~> 5.0"`)
2. Fetch schema from provider docs matching the version

| Provider | Documentation URL |
|----------|-------------------|
| Cloudflare | `https://raw.githubusercontent.com/cloudflare/terraform-provider-cloudflare/{tag}/docs/resources/{resource}.md` |
| Google | `https://raw.githubusercontent.com/hashicorp/terraform-provider-google/{tag}/website/docs/r/{resource}.html.markdown` |
| GitHub | `https://raw.githubusercontent.com/integrations/terraform-provider-github/{tag}/website/docs/r/{resource}.html.markdown` |

Use `main` if no version constraint specified, otherwise use matching tag (e.g., `v5.0.0`).

## Key Points

| Check | Action |
|-------|--------|
| Required attributes | Ensure all required attributes are set |
| Attribute types | Match expected types (string, list, map, block) |
| Valid values | Use only documented enum values |
| Deprecated attributes | Avoid deprecated attributes, use replacements |
