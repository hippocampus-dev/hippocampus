package spotify

//go:generate bash -c "set -o pipefail; curl -fsSL https://developer.spotify.com/reference/web-api/open-api-schema.yaml | sed -E '/^        - (available_markets|is_playable|audio_preview_url|external_ids|genres|label|popularity)$/d;/^      x-spotify-policy-list:/,+1d;/^              required: true$/d' | ogen -config .ogen.yml -target . -clean -package spotify /dev/stdin"
