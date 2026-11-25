# JavaScript Testing Patterns

How to write consistent tests in TypeScript packages.

## Framework Choice

| Test Kind | Framework | File Suffix | Example |
|-----------|-----------|-------------|---------|
| Unit | `vitest` | `.test.ts` | `google-app-scripts/sheethub/test/parser/csv.test.ts` |
| End-to-end | `@playwright/test` | `.spec.ts` | `cluster/applications/csviewer/tests/csv-parsing/should-parse-basic-csv.spec.ts` |

Do not mix suffixes: `.test.ts` is exclusively vitest, `.spec.ts` is exclusively Playwright.

## Unit Test Structure

```typescript
import { describe, expect, test } from "vitest";
import { parseRFC4180 } from "../../src/parser/csv";

describe("parseRFC4180", () => {
  test("simple", () => {
    expect(
      parseRFC4180(`\
a,b,c
1,2,3
`),
    ).toStrictEqual([
      ["a", "b", "c"],
      ["1", "2", "3"],
    ]);
  });
});
```

| Practice | Reason |
|----------|--------|
| Top-level `describe` per exported symbol | Group cases per unit under test |
| `test/` mirrors `src/` directory layout | Locate the test for a source file by path |

## E2E Test Structure

```typescript
import { expect, test } from "@playwright/test";

test.describe("CSV Parsing - RFC 4180 Compliance", () => {
  test("should parse basic CSV with headers and rows", async ({ page }) => {
    await page.goto("/");
    // assertions
  });
});
```

| Practice | Reason |
|----------|--------|
| File name from the test plan's `**File:**` line for a spec the Playwright test generator agent writes (`cluster/applications/csviewer/specs/csviewer-test-plan.md` gives `should-{behavior}.spec.ts`) | The agent saves each scenario to the path its plan names, so a hand rename leaves that `**File:**` line pointing at a missing file |
| File name `{subject}.spec.ts` (kebab-case) for a hand-written spec, naming what it checks within its feature area (`extension/tests/compatibility/playwright.spec.ts`) | `tests/{feature-area}/` already names what kind of check the spec makes, so the file name only has to add its subject |
| Grouped under `tests/{feature-area}/` | Run a feature-scoped subset via Playwright path filter |
| `test.describe` wraps related specs | Shared setup scope in the e2e harness |
