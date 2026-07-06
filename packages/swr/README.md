# swr

<!-- TOC -->
* [swr](#swr)
<!-- TOC -->

swr is a Stale-While-Revalidate cache that returns stale values immediately while refreshing in the background, using [singleflight](../singleflight) for deduplication.
