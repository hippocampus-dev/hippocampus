# CSViewer Test Plan

## Overview

CSViewer is a lightweight web-based CSV file viewer built with Preact and TailwindCSS. It provides interactive CSV browsing with fuzzy search capabilities, markdown link rendering, and URL-based state management.

**Key Features:**
- RFC 4180-compliant CSV parser (custom implementation)
- Bitap fuzzy search algorithm with configurable edit distance
- Japanese text segmentation via Intl.Segmenter
- i18n support for localized columns based on browser language
- Markdown link parsing and rendering within CSV cells
- URL-based state management for file selection and row highlighting

**Architecture:**
- Zero-build architecture (CDN dependencies)
- Preact from cdn.skypack.dev
- TailwindCSS from cdn.tailwindcss.com

---

## Test Suites

### 1. Initial Load and File Selection

**Seed File:** `seed.spec.ts`

#### 1.1 should load default file on initial page visit
**Steps:**
1. Navigate to the application root URL without any query parameters
2. Wait for the page to fully load

**Expected Results:**
- The first file defined in the mapping configuration is loaded
- The corresponding file button shows active state styling
- CSV data is displayed in the table

**File:** `tests/initial-load/should-load-default-file.spec.ts`

---

#### 1.2 should load specific file when URL contains file parameter
**Steps:**
1. Navigate to the application with `?file=<filename>` query parameter
2. Wait for the page to fully load

**Expected Results:**
- The specified file is loaded and displayed
- The corresponding file button shows active state
- URL parameter is preserved

**File:** `tests/initial-load/should-load-file-from-url-param.spec.ts`

---

#### 1.3 should handle non-existent file parameter gracefully
**Steps:**
1. Navigate to the application with `?file=nonexistent` query parameter
2. Wait for the page to load

**Expected Results:**
- Application does not crash
- Error state or fallback to default file is displayed
- User-friendly error message if applicable

**File:** `tests/initial-load/should-handle-nonexistent-file.spec.ts`

---

#### 1.4 should switch files when clicking file buttons
**Steps:**
1. Navigate to the application
2. Wait for initial file to load
3. Click on a different file button

**Expected Results:**
- New file data is loaded and displayed
- URL is updated with new file parameter
- Previous file button loses active state
- Clicked button gains active state

**File:** `tests/initial-load/should-switch-files-on-button-click.spec.ts`

---

#### 1.5 should prefetch file on button hover
**Steps:**
1. Navigate to the application
2. Hover over a file button (not the currently active one)
3. Monitor network requests

**Expected Results:**
- Network request is initiated for the hovered file's CSV
- Prefetch happens before click action

**File:** `tests/initial-load/should-prefetch-on-hover.spec.ts`

---

### 2. CSV Parsing - RFC 4180 Compliance

**Seed File:** `seed.spec.ts`

#### 2.1 should parse basic CSV with headers and rows
**Steps:**
1. Load a CSV file with simple comma-separated values
2. Observe the rendered table

**Expected Results:**
- Headers are correctly extracted from first row
- All data rows are parsed correctly
- Column count matches header count

**File:** `tests/csv-parsing/should-parse-basic-csv.spec.ts`

---

#### 2.2 should handle escaped double quotes within quoted fields
**Steps:**
1. Load a CSV containing fields like `"He said ""Hello"""`
2. Observe the rendered cell content

**Expected Results:**
- Escaped quotes (`""`) are rendered as single quotes (`"`)
- Field content is correctly displayed

**File:** `tests/csv-parsing/should-handle-escaped-quotes.spec.ts`

---

#### 2.3 should handle newlines within quoted fields
**Steps:**
1. Load a CSV containing multi-line quoted fields
2. Observe the rendered cell content

**Expected Results:**
- Newlines within quoted fields are preserved
- Field is not split into multiple rows
- Line breaks render correctly in the cell

**File:** `tests/csv-parsing/should-handle-newlines-in-quotes.spec.ts`

---

#### 2.4 should handle commas within quoted fields
**Steps:**
1. Load a CSV containing fields like `"Hello, World"`
2. Observe the rendered cell content

**Expected Results:**
- Commas within quoted fields do not split the field
- Full content including comma is displayed in single cell

**File:** `tests/csv-parsing/should-handle-commas-in-quotes.spec.ts`

---

#### 2.5 should handle various line ending formats (CRLF, LF, CR)
**Steps:**
1. Load CSV files with different line ending formats
2. Observe row parsing

**Expected Results:**
- CRLF (`\r\n`) line endings are handled correctly
- LF (`\n`) line endings are handled correctly
- CR (`\r`) line endings are handled correctly

**File:** `tests/csv-parsing/should-handle-line-endings.spec.ts`

---

#### 2.6 should handle trailing newline at end of file
**Steps:**
1. Load a CSV file with trailing newline(s)
2. Count the rendered rows

**Expected Results:**
- Trailing newlines do not create empty rows
- Row count matches actual data rows

**File:** `tests/csv-parsing/should-handle-trailing-newline.spec.ts`

---

#### 2.7 should handle malformed CSV gracefully
**Steps:**
1. Load a CSV with RFC 4180 violations (unclosed quotes, etc.)
2. Observe application behavior

**Expected Results:**
- Application does not crash
- Best-effort parsing is attempted
- Error indication if parsing fails completely

**File:** `tests/csv-parsing/should-handle-malformed-csv.spec.ts`

---

### 3. Search Functionality - Basic Search

**Seed File:** `seed.spec.ts`

#### 3.1 should find exact matches in indexed columns
**Steps:**
1. Load a CSV file with indexed columns defined in mapping
2. Enter an exact match search term
3. Observe filtered results

**Expected Results:**
- Only rows containing the exact term are displayed
- Match is found in indexed columns only
- Non-matching rows are hidden

**File:** `tests/search-basic/should-find-exact-matches.spec.ts`

---

#### 3.2 should perform case-insensitive search
**Steps:**
1. Load a CSV file
2. Enter a search term with different case than data
3. Observe filtered results

**Expected Results:**
- Matches are found regardless of case
- "ABC" matches "abc", "Abc", etc.

**File:** `tests/search-basic/should-be-case-insensitive.spec.ts`

---

#### 3.3 should only search in indexed columns
**Steps:**
1. Load a CSV with some columns marked as indexed and others not
2. Enter a term that exists only in non-indexed columns
3. Observe filtered results

**Expected Results:**
- No matches found for terms in non-indexed columns
- Only indexed columns are searchable

**File:** `tests/search-basic/should-search-only-indexed-columns.spec.ts`

---

#### 3.4 should handle multiple search terms separated by spaces
**Steps:**
1. Load a CSV file
2. Enter multiple space-separated search terms
3. Observe filtered results

**Expected Results:**
- Rows must match ALL search terms (AND logic)
- Each term can match different columns
- Only rows matching all terms are displayed

**File:** `tests/search-basic/should-handle-multiple-terms.spec.ts`

---

#### 3.5 should show all rows when search query is empty
**Steps:**
1. Load a CSV file
2. Clear the search input (or leave empty)
3. Observe displayed rows

**Expected Results:**
- All rows from the CSV are displayed
- No filtering is applied

**File:** `tests/search-basic/should-show-all-rows-when-empty.spec.ts`

---

#### 3.6 should trim whitespace-only queries
**Steps:**
1. Load a CSV file
2. Enter only whitespace characters in search
3. Observe displayed rows

**Expected Results:**
- Whitespace-only query is treated as empty
- All rows are displayed

**File:** `tests/search-basic/should-trim-whitespace-queries.spec.ts`

---

#### 3.7 should display no results message when no matches found
**Steps:**
1. Load a CSV file
2. Enter a search term that matches no rows
3. Observe the display

**Expected Results:**
- Table body is empty or shows "no results" indicator
- Application does not crash
- Clear indication that no matches were found

**File:** `tests/search-basic/should-show-no-results.spec.ts`

---

### 4. Search Functionality - Fuzzy Search (Bitap Algorithm)

**Seed File:** `seed.spec.ts`

#### 4.1 should allow one character edit when maxDistance is 1
**Steps:**
1. Load a CSV with column configured with `maxDistance: 1`
2. Enter a search term with one character different from actual data
3. Observe filtered results

**Expected Results:**
- Rows with 1-edit-distance matches are found
- Substitution, insertion, deletion within 1 edit are matched

**File:** `tests/search-fuzzy/should-allow-one-edit-distance.spec.ts`

---

#### 4.2 should require exact match when maxDistance is 0
**Steps:**
1. Load a CSV with column configured with `maxDistance: 0`
2. Enter a search term with one character different
3. Observe filtered results

**Expected Results:**
- Only exact matches are found
- Fuzzy matching is disabled for this column

**File:** `tests/search-fuzzy/should-require-exact-match-when-zero.spec.ts`

---

#### 4.3 should respect maxDistance limit per column
**Steps:**
1. Load a CSV with different maxDistance values per column
2. Test fuzzy searches against each column
3. Observe filtered results

**Expected Results:**
- Each column respects its own maxDistance setting
- Column A with maxDistance=2 allows 2 edits
- Column B with maxDistance=1 allows only 1 edit

**File:** `tests/search-fuzzy/should-respect-per-column-distance.spec.ts`

---

#### 4.4 should support Japanese text segmentation
**Steps:**
1. Load a CSV containing Japanese text
2. Enter a Japanese search term
3. Observe filtered results

**Expected Results:**
- Japanese text is properly segmented using Intl.Segmenter
- Fuzzy matching works with Japanese characters
- Partial matches within segmented words are found

**File:** `tests/search-fuzzy/should-support-japanese-segmentation.spec.ts`

---

#### 4.5 should handle pattern length limit (64 characters)
**Steps:**
1. Load a CSV file
2. Enter a search term longer than 64 characters
3. Observe behavior

**Expected Results:**
- Search still functions (possibly truncated or exact match only)
- Application does not crash
- Bitap algorithm limitation is handled gracefully

**File:** `tests/search-fuzzy/should-handle-pattern-length-limit.spec.ts`

---

#### 4.6 should handle negative maxDistance gracefully
**Steps:**
1. Configure a column with negative maxDistance (if possible via config)
2. Perform a search

**Expected Results:**
- Application does not crash
- Treated as 0 or ignored
- Error handling is in place

**File:** `tests/search-fuzzy/should-handle-negative-max-distance.spec.ts`

---

### 5. Markdown Link Rendering

**Seed File:** `seed.spec.ts`

#### 5.1 should convert markdown links to clickable hyperlinks
**Steps:**
1. Load a CSV containing cells with `[text](url)` format
2. Observe the rendered cell content

**Expected Results:**
- Markdown links are rendered as `<a>` elements
- Link text is displayed
- href points to the URL

**File:** `tests/markdown/should-convert-links-to-hyperlinks.spec.ts`

---

#### 5.2 should open links in new tab
**Steps:**
1. Load a CSV with markdown links
2. Click on a rendered link
3. Observe browser behavior

**Expected Results:**
- Link opens in a new tab/window
- `target="_blank"` attribute is present
- `rel="noopener noreferrer"` for security

**File:** `tests/markdown/should-open-links-in-new-tab.spec.ts`

---

#### 5.3 should handle multiple links in same cell
**Steps:**
1. Load a CSV with cells containing multiple markdown links
2. Observe the rendered cell content

**Expected Results:**
- All links in the cell are rendered as hyperlinks
- Links are separated by surrounding text
- Each link is independently clickable

**File:** `tests/markdown/should-handle-multiple-links.spec.ts`

---

#### 5.4 should render plain text without markdown unchanged
**Steps:**
1. Load a CSV with cells containing plain text (no markdown)
2. Observe the rendered cell content

**Expected Results:**
- Plain text is displayed as-is
- No transformation or escaping issues
- Special characters are preserved

**File:** `tests/markdown/should-render-plain-text.spec.ts`

---

#### 5.5 should style links appropriately
**Steps:**
1. Load a CSV with markdown links
2. Observe link styling

**Expected Results:**
- Links have distinct styling (color, underline)
- Links are visually identifiable as clickable
- Accessible focus states are present

**File:** `tests/markdown/should-style-links.spec.ts`

---

#### 5.6 should handle special characters in URLs
**Steps:**
1. Load a CSV with links containing encoded URLs
2. Observe link functionality

**Expected Results:**
- URL-encoded characters are preserved
- Links navigate to correct destinations
- Query parameters and fragments work

**File:** `tests/markdown/should-handle-special-url-chars.spec.ts`

---

#### 5.7 should handle malformed markdown gracefully
**Steps:**
1. Load a CSV with malformed markdown (unclosed brackets, etc.)
2. Observe the rendered cell content

**Expected Results:**
- Application does not crash
- Malformed markdown is displayed as plain text
- No partial rendering artifacts

**File:** `tests/markdown/should-handle-malformed-markdown.spec.ts`

---

### 6. URL-Based State Management and Row Highlighting

**Seed File:** `seed.spec.ts`

#### 6.1 should highlight row when URL contains hash
**Steps:**
1. Navigate to application with URL hash (e.g., `#row-identifier`)
2. Wait for page to load

**Expected Results:**
- Corresponding row is visually highlighted
- Highlight styling is distinct and visible
- Row remains highlighted until changed

**File:** `tests/url-state/should-highlight-row-from-hash.spec.ts`

---

#### 6.2 should update URL hash when row is selected
**Steps:**
1. Load the application
2. Click on a table row
3. Observe the URL

**Expected Results:**
- URL hash is updated to reflect selected row
- Hash contains row identifier
- Browser history entry is created (or replaced)

**File:** `tests/url-state/should-update-hash-on-row-select.spec.ts`

---

#### 6.3 should scroll to highlighted row smoothly
**Steps:**
1. Navigate to application with hash pointing to row not in viewport
2. Observe scroll behavior

**Expected Results:**
- Page scrolls to bring highlighted row into view
- Scroll is smooth (not jarring jump)
- Row is centered or visible in viewport

**File:** `tests/url-state/should-scroll-to-highlighted-row.spec.ts`

---

#### 6.4 should preserve hash when switching files
**Steps:**
1. Navigate to application with hash
2. Switch to a different file via button click
3. Observe URL and highlighting

**Expected Results:**
- Hash is cleared or updated appropriately
- No stale highlighting from previous file
- URL reflects new file selection

**File:** `tests/url-state/should-handle-hash-on-file-switch.spec.ts`

---

#### 6.5 should URL-encode row identifiers properly
**Steps:**
1. Load CSV with special characters in row identifiers
2. Click on a row with special characters
3. Observe the URL hash

**Expected Results:**
- Special characters are properly URL-encoded
- Hash can be copied and shared
- Navigating to encoded URL works correctly

**File:** `tests/url-state/should-url-encode-identifiers.spec.ts`

---

#### 6.6 should preserve file parameter when selecting rows
**Steps:**
1. Navigate to application with `?file=<name>`
2. Click on a row
3. Observe the URL

**Expected Results:**
- File query parameter is preserved
- Both file and hash are in URL
- Full state is shareable via URL

**File:** `tests/url-state/should-preserve-file-param.spec.ts`

---

#### 6.7 should handle non-existent hash gracefully
**Steps:**
1. Navigate to application with hash that doesn't match any row
2. Observe behavior

**Expected Results:**
- Application does not crash
- No row is highlighted
- Page loads normally without errors

**File:** `tests/url-state/should-handle-nonexistent-hash.spec.ts`

---

### 7. Internationalization (i18n) Support

**Seed File:** `seed.spec.ts`

#### 7.1 should select localized column based on browser language
**Steps:**
1. Set browser language to Japanese (ja)
2. Load CSV with both base and ja-localized columns
3. Observe displayed columns

**Expected Results:**
- Japanese localized column is displayed
- Base column is hidden/replaced
- Column header shows localized name

**File:** `tests/i18n/should-select-localized-column.spec.ts`

---

#### 7.2 should fallback to base column when localization unavailable
**Steps:**
1. Set browser language to unsupported language (e.g., fr)
2. Load CSV with no French localization
3. Observe displayed columns

**Expected Results:**
- Base column is displayed as fallback
- No errors or missing columns
- Application functions normally

**File:** `tests/i18n/should-fallback-to-base-column.spec.ts`

---

#### 7.3 should respect per-column i18n configuration in mapping
**Steps:**
1. Configure mapping with i18n enabled for some columns only
2. Load CSV with localized data
3. Observe column display

**Expected Results:**
- Only columns with i18n enabled show localized versions
- Other columns display base data
- Mixed display works correctly

**File:** `tests/i18n/should-respect-per-column-config.spec.ts`

---

#### 7.4 should handle multiple browser languages with priority
**Steps:**
1. Set browser with multiple languages (e.g., `ja, en, de`)
2. Load CSV with some but not all localizations
3. Observe column selection

**Expected Results:**
- First available localization is used
- Priority order is respected
- Fallback chain works correctly

**File:** `tests/i18n/should-handle-language-priority.spec.ts`

---

#### 7.5 should normalize language codes (hyphen splitting)
**Steps:**
1. Set browser language with region (e.g., `ja-JP`, `en-US`)
2. Load CSV with base language localizations (ja, en)
3. Observe column selection

**Expected Results:**
- `ja-JP` matches `ja` localization
- Region suffix is handled correctly
- Base language code is extracted

**File:** `tests/i18n/should-normalize-language-codes.spec.ts`

---

#### 7.6 should apply i18n only to stored/displayed columns
**Steps:**
1. Configure mapping with stored columns and i18n
2. Load CSV
3. Verify i18n affects only relevant columns

**Expected Results:**
- Only columns in stored/display list are affected
- Index-only columns are not localized
- Search still works on base data

**File:** `tests/i18n/should-apply-to-stored-columns-only.spec.ts`

---

### 8. Table Display and UI Elements

**Seed File:** `seed.spec.ts`

#### 8.1 should display table headers from stored column names
**Steps:**
1. Load a CSV file
2. Observe the table header row

**Expected Results:**
- Column headers match stored column configuration
- Headers are displayed in correct order
- Header text is readable

**File:** `tests/ui/should-display-table-headers.spec.ts`

---

#### 8.2 should display table rows with stored column data
**Steps:**
1. Load a CSV file
2. Observe the table body rows

**Expected Results:**
- Each row displays correct column data
- Columns align with headers
- All stored columns are visible

**File:** `tests/ui/should-display-table-rows.spec.ts`

---

#### 8.3 should display empty cells correctly
**Steps:**
1. Load a CSV with empty cells
2. Observe empty cell rendering

**Expected Results:**
- Empty cells are rendered (not collapsed)
- Table structure is maintained
- No visual artifacts

**File:** `tests/ui/should-display-empty-cells.spec.ts`

---

#### 8.4 should wrap long text and preserve line breaks
**Steps:**
1. Load a CSV with long text content
2. Observe cell text rendering

**Expected Results:**
- Long text wraps within cell boundaries
- Line breaks in data are preserved
- No horizontal overflow issues

**File:** `tests/ui/should-wrap-long-text.spec.ts`

---

#### 8.5 should apply hover styling on rows
**Steps:**
1. Load a CSV file
2. Hover over table rows
3. Observe styling changes

**Expected Results:**
- Hovered row has distinct background color
- Hover state is clearly visible
- Styling is removed on mouse leave

**File:** `tests/ui/should-apply-hover-styling.spec.ts`

---

#### 8.6 should style search box with focus states
**Steps:**
1. Load the application
2. Focus the search input
3. Observe styling changes

**Expected Results:**
- Search box has visible focus indicator
- Focus ring or border change is present
- Accessible focus states

**File:** `tests/ui/should-style-search-focus.spec.ts`

---

#### 8.7 should show active state on selected file button
**Steps:**
1. Load the application
2. Observe file selection buttons
3. Click different buttons

**Expected Results:**
- Active file button has distinct styling
- Only one button shows active state
- State changes on file switch

**File:** `tests/ui/should-show-active-file-button.spec.ts`

---

#### 8.8 should provide accessible focus states on file buttons
**Steps:**
1. Load the application
2. Tab through file buttons
3. Observe focus indicators

**Expected Results:**
- Each button has visible focus indicator
- Focus order is logical
- Keyboard navigation works

**File:** `tests/ui/should-have-accessible-focus.spec.ts`

---

#### 8.9 should maintain responsive layout
**Steps:**
1. Load the application at various viewport sizes
2. Resize browser window
3. Observe layout adjustments

**Expected Results:**
- Layout adapts to viewport width
- Content remains centered
- No horizontal scrollbar on standard widths

**File:** `tests/ui/should-maintain-responsive-layout.spec.ts`

---

### 9. Performance and Edge Cases

**Seed File:** `seed.spec.ts`

#### 9.1 should load large CSV files efficiently
**Steps:**
1. Load a CSV file with 10,000+ rows
2. Measure load time
3. Observe UI responsiveness

**Expected Results:**
- File loads within acceptable time (< 5 seconds)
- UI remains responsive during load
- Memory usage is reasonable

**File:** `tests/performance/should-load-large-files.spec.ts`

---

#### 9.2 should search large datasets efficiently
**Steps:**
1. Load a large CSV file
2. Perform search operations
3. Measure response time

**Expected Results:**
- Search results appear quickly (< 500ms)
- UI remains responsive during search
- No freezing or blocking

**File:** `tests/performance/should-search-large-datasets.spec.ts`

---

#### 9.3 should handle network errors during CSV fetch
**Steps:**
1. Simulate network failure
2. Attempt to load a CSV file
3. Observe error handling

**Expected Results:**
- Error message is displayed
- Application does not crash
- Retry or recovery option available

**File:** `tests/performance/should-handle-network-errors.spec.ts`

---

#### 9.4 should abort pending fetch when switching files
**Steps:**
1. Start loading a large CSV file
2. Before load completes, switch to another file
3. Observe network behavior

**Expected Results:**
- Previous fetch is aborted
- New file loads correctly
- No race conditions or stale data

**File:** `tests/performance/should-abort-pending-fetch.spec.ts`

---

#### 9.5 should handle CSV with inconsistent column counts
**Steps:**
1. Load a CSV where some rows have different column counts
2. Observe parsing and display

**Expected Results:**
- Application handles gracefully
- Missing columns show empty values
- Extra columns are ignored or handled

**File:** `tests/performance/should-handle-inconsistent-columns.spec.ts`

---

#### 9.6 should manage memory on repeated file switches
**Steps:**
1. Load and switch between files multiple times
2. Monitor memory usage
3. Check for memory leaks

**Expected Results:**
- Memory usage remains stable
- Previous file data is garbage collected
- No memory leaks after multiple switches

**File:** `tests/performance/should-manage-memory.spec.ts`

---

#### 9.7 should handle BOM (Byte Order Mark) in CSV files
**Steps:**
1. Load a CSV file with UTF-8 BOM
2. Observe first cell/header content

**Expected Results:**
- BOM is stripped from content
- First header/cell is clean
- No invisible characters displayed

**File:** `tests/performance/should-handle-bom.spec.ts`

---

#### 9.8 should handle special characters in CSV content
**Steps:**
1. Load CSV with various special characters (emoji, unicode, etc.)
2. Observe cell rendering

**Expected Results:**
- All special characters render correctly
- Emoji display properly
- Unicode characters are preserved

**File:** `tests/performance/should-handle-special-chars.spec.ts`

---

#### 9.9 should handle empty CSV file
**Steps:**
1. Load an empty CSV file (or headers only)
2. Observe application behavior

**Expected Results:**
- Application handles empty file gracefully
- Empty state indicator shown
- No errors or crashes

**File:** `tests/performance/should-handle-empty-csv.spec.ts`

---

### 10. Browser Compatibility and CDN Dependencies

**Seed File:** `seed.spec.ts`

#### 10.1 should load Preact from CDN successfully
**Steps:**
1. Load the application
2. Monitor network requests to cdn.skypack.dev
3. Verify Preact initialization

**Expected Results:**
- Preact library loads successfully
- No CDN errors in console
- Application renders correctly

**File:** `tests/compatibility/should-load-preact-cdn.spec.ts`

---

#### 10.2 should load TailwindCSS from CDN successfully
**Steps:**
1. Load the application
2. Monitor network requests to cdn.tailwindcss.com
3. Verify styles are applied

**Expected Results:**
- TailwindCSS loads successfully
- Utility classes are functional
- Styling is applied correctly

**File:** `tests/compatibility/should-load-tailwind-cdn.spec.ts`

---

#### 10.3 should function with ES modules browser support
**Steps:**
1. Load the application in modern browser
2. Verify module loading via `<script type="module">`
3. Check for module errors

**Expected Results:**
- ES modules load correctly
- Import statements function
- No module resolution errors

**File:** `tests/compatibility/should-support-es-modules.spec.ts`

---

#### 10.4 should verify Intl.Segmenter API availability
**Steps:**
1. Load the application
2. Test Japanese text search
3. Verify segmentation works

**Expected Results:**
- Intl.Segmenter is available
- Japanese text is properly segmented
- Fallback exists if API unavailable

**File:** `tests/compatibility/should-have-intl-segmenter.spec.ts`

---

#### 10.5 should degrade gracefully on CDN failure
**Steps:**
1. Block CDN requests
2. Attempt to load application
3. Observe error handling

**Expected Results:**
- Error message is displayed
- Application shows fallback state
- User is informed of the issue

**File:** `tests/compatibility/should-handle-cdn-failure.spec.ts`

---

## Test Summary

| Suite | Test Count |
|-------|------------|
| 1. Initial Load and File Selection | 5 |
| 2. CSV Parsing - RFC 4180 Compliance | 7 |
| 3. Search Functionality - Basic Search | 7 |
| 4. Search Functionality - Fuzzy Search | 6 |
| 5. Markdown Link Rendering | 7 |
| 6. URL-Based State Management | 7 |
| 7. Internationalization (i18n) | 6 |
| 8. Table Display and UI Elements | 9 |
| 9. Performance and Edge Cases | 9 |
| 10. Browser Compatibility | 5 |
| **Total** | **68** |

---

## Test Environment

- **Browser:** Chromium (via Playwright)
- **Base URL:** Configured in Playwright config
- **Seed File:** `seed.spec.ts`
- **Test Framework:** Playwright Test

## Notes

- All tests assume a clean/fresh state (new browser context)
- Tests should be independent and runnable in any order
- Network conditions may be mocked for error handling tests
- Performance thresholds are guidelines and may need adjustment
