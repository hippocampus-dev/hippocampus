import {stringifyYAML} from "../../src/parser/yaml";
import {describe, expect, test} from "vitest";

describe("stringifyYAML", () => {
    test("simple", () => {
        expect(stringifyYAML([["key", "value"], [1, "a"], [2, "b"]])).toBe(`\
- key: 1
  value: a
- key: 2
  value: b
`);
    });

    test("with array", () => {
        expect(stringifyYAML([["key", "value", "options"], [1, "a", "- foo\n- bar\n"], [2, "b", ""]])).toBe(`\
- key: 1
  value: a
  options:
    - foo
    - bar
- key: 2
  value: b
`);
    });

    test("with nested keys", () => {
        expect(stringifyYAML([["key", "value", "options.foo"], [1, "a", "bar"], [2, "b", ""]])).toBe(`\
- key: 1
  value: a
  options:
    foo: bar
- key: 2
  value: b
  options: {}
`);
    });

    test("with multiline string", () => {
        expect(stringifyYAML([["key", "value"], [1, "a\nb"], [2, "c"]])).toBe(`\
- key: 1
  value: |-
    a
    b
- key: 2
  value: c
`);
    });

    test("with path", () => {
        expect(stringifyYAML([["key", "value"], [1, "a"], [2, "b"]], "key")).toBe(`\
key:
  - key: 1
    value: a
  - key: 2
    value: b
`);
    });

    test("with nested path", () => {
        expect(stringifyYAML([["key", "value"], [1, "a"], [2, "b"]], "nested.key")).toBe(`\
nested:
  key:
    - key: 1
      value: a
    - key: 2
      value: b
`);
    });
});
