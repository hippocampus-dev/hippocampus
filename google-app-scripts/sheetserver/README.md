# sheetserver

<!-- TOC -->
* [sheetserver](#sheetserver)
  * [Development](#development)
  * [Deployment](#deployment)
<!-- TOC -->

sheetserver is a Google Apps Script proxy for exporting Google Sheets data, bypassing redirect incompatibility with Deno.

## Development

```sh
$ make all
```

## Deployment

```sh
$ clasp login
$ npm ci
$ npm run deploy
```
