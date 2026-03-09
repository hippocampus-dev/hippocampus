import {parseYAML, stringifyYAML, parseYAMLDocument, stringifyYAMLDocument} from "../../src/parser/yaml";
import {describe, test, expect} from "vitest";

describe("parseYAML", () => {
    test("simple", () => {
        expect(parseYAML(`\
- key: 1
  value: a
- key: 2
  value: b
`)
        ).toStrictEqual([["key", "value"], [1, "a"], [2, "b"]]);
    });

    test("with array", () => {
        expect(parseYAML(`\
- key: 1
  value: a
  options:
    - foo
    - bar
- key: 2
  value: b
  `)
        ).toStrictEqual([["key", "value", "options"], [1, "a", "- foo\n- bar\n"], [2, "b", ""]]);
    });

    test("with nested keys", () => {
        expect(parseYAML(`\
- key: 1
  value: a
  options:
    foo: bar
- key: 2
  value: b
`)
        ).toStrictEqual([["key", "value", "options.foo"], [1, "a", "bar"], [2, "b", ""]]);
    });

    test("with array in nested key", () => {
        expect(parseYAML(`\
- key: 1
  value: a
  options:
    foo: bar
    baz:
      - 1
      - 2
      - 3
- key: 2
  value: b
`)
        ).toStrictEqual([["key", "value", "options.foo", "options.baz"], [1, "a", "bar", "- 1\n- 2\n- 3\n"], [2, "b", "", ""]]);
    });

    test("invalid", () => {
        expect(() => {
            parseYAML(`\
key:
  - key: 1
    value: a
  - key: 2
    value: b
`)
        }).toThrow(/Invalid YAML/);
    });

    test("with path", () => {
        expect(parseYAML(`\
key:
  - key: 1
    value: a
  - key: 2
    value: b
`, "key")
        ).toStrictEqual([["key", "value"], [1, "a"], [2, "b"]]);
    });

    test("with nested path", () => {
        expect(parseYAML(`\
nested:
  key:
    - key: 1
      value: a
    - key: 2
      value: b
`, "nested.key")
        ).toStrictEqual([["key", "value"], [1, "a"], [2, "b"]]);
    });
});

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

describe("parseYAMLDocument", () => {
    test("simple", () => {
        expect(parseYAMLDocument(`\
- key: 1
  value: a
- key: 2
  value: b
`)
        ).toStrictEqual([[["_comment", undefined], ["key", undefined], ["value", undefined]], [["", undefined], [1, undefined], ["a", undefined]], [["", undefined], [2, undefined], ["b", undefined]]]);
    });

    test("with array", () => {
        expect(parseYAMLDocument(`\
- key: 1
  value: a
  options:
    - foo
    - bar
- key: 2
  value: b
`)
        ).toStrictEqual([[["_comment", undefined], ["key", undefined], ["value", undefined], ["options", undefined]], [["", undefined], [1, undefined], ["a", undefined], ["- foo\n- bar\n", undefined]], [["", undefined], [2, undefined], ["b", undefined], ["", undefined]]]);
    });

    test("with nested keys", () => {
        expect(parseYAMLDocument(`\
- key: 1
  value: a
  options:
    foo: bar
- key: 2
  value: b
`)
        ).toStrictEqual([[["_comment", undefined], ["key", undefined], ["value", undefined], ["options.foo", undefined]], [["", undefined], [1, undefined], ["a", undefined], ["bar", undefined]], [["", undefined], [2, undefined], ["b", undefined], ["", undefined]]]);
    });

    test("with array in nested key", () => {
        expect(parseYAMLDocument(`\
- key: 1
  value: a
  options:
    foo: bar
    baz:
      - 1
      - 2
      - 3
- key: 2
  value: b
`)
        ).toStrictEqual([[["_comment", undefined], ["key", undefined], ["value", undefined], ["options.foo", undefined], ["options.baz", undefined]], [["", undefined], [1, undefined], ["a", undefined], ["bar", undefined], ["- 1\n- 2\n- 3\n", undefined]], [["", undefined], [2, undefined], ["b", undefined], ["", undefined], ["", undefined]]]);
    });

    test("invalid", () => {
        expect(() => {
            parseYAML(`\
key:
  - key: 1
    value: a
  - key: 2
    value: b
`)
        }).toThrow(/Invalid YAML/);
    });

    test("with path", () => {
        expect(parseYAMLDocument(`\
key:
  - key: 1
    value: a
  - key: 2
    value: b
`, "key")
        ).toStrictEqual([[["_comment", undefined], ["key", undefined], ["value", undefined]], [["", undefined], [1, undefined], ["a", undefined]], [["", undefined], [2, undefined], ["b", undefined]]]);
    });

    test("with nested path", () => {
        expect(parseYAMLDocument(`\
nested:
    key:
      - key: 1
        value: a
      - key: 2
        value: b
`, "nested.key")
        ).toStrictEqual([[["_comment", undefined], ["key", undefined], ["value", undefined]], [["", undefined], [1, undefined], ["a", undefined]], [["", undefined], [2, undefined], ["b", undefined]]]);
    });

    test("with comment", () => {
        expect(parseYAMLDocument(`\
# comment: 1
- key: 1
  value: a
  options:
    - foo # comment: foo
    - bar
# comment: 2
- key: 2
  value: b # this is value comment: b
`)
        ).toStrictEqual([[["_comment", undefined], ["key", undefined], ["value", undefined], ["options", undefined]], [["", " comment: 1"], [1, undefined], ["a", undefined], ["- foo # comment: foo\n- bar\n", undefined]], [["", " comment: 2"], [2, undefined], ["b", " this is value comment: b"], ["", undefined]]]);
    });
});

describe("stringifyYAMLDocument", () => {
    test("simple", () => {
        expect(stringifyYAMLDocument([[["_comment", undefined], ["key", undefined], ["value", undefined]], [["", undefined], [1, undefined], ["a", undefined]], [["", undefined], [2, undefined], ["b", undefined]]])).toBe(`\
- key: 1
  value: a
- key: 2
  value: b
`);
    });

    test("with array", () => {
        expect(stringifyYAMLDocument([[["_comment", undefined], ["key", undefined], ["value", undefined], ["options", undefined]], [["", undefined], [1, undefined], ["a", undefined], ["- foo\n- bar\n", undefined]], [["", undefined], [2, undefined], ["b", undefined], ["", undefined]]])).toBe(`\
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
        expect(stringifyYAMLDocument([[["_comment", undefined], ["key", undefined], ["value", undefined], ["options.foo", undefined]], [["", undefined], [1, undefined], ["a", undefined], ["bar", undefined]], [["", undefined], [2, undefined], ["b", undefined], ["", undefined]]])).toBe(`\
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
        expect(stringifyYAMLDocument([[["_comment", undefined], ["key", undefined], ["value", undefined]], [["", undefined], [1, undefined], ["a\nb", undefined]], [["", undefined], [2, undefined], ["c", undefined]]])).toBe(`\
- key: 1
  value: |-
    a
    b
- key: 2
  value: c
`);
    });

    test("with path", () => {
        expect(stringifyYAMLDocument([[["_comment", undefined], ["key", undefined], ["value", undefined]], [["", undefined], [1, undefined], ["a", undefined]], [["", undefined], [2, undefined], ["b", undefined]]], "key")).toBe(`\
key:
  - key: 1
    value: a
  - key: 2
    value: b
`);
    });

    test("with nested path", () => {
        expect(stringifyYAMLDocument([[["_comment", undefined], ["key", undefined], ["value", undefined]], [["", undefined], [1, undefined], ["a", undefined]], [["", undefined], [2, undefined], ["b", undefined]]], "nested.key")).toBe(`\
nested:
  key:
    - key: 1
      value: a
    - key: 2
      value: b
`);
    });

    test("with comment", () => {
        expect(stringifyYAMLDocument([[["_comment", undefined], ["key", undefined], ["value", undefined], ["options", undefined]], [["", " comment: 1"], [1, undefined], ["a", undefined], ["- foo # comment: foo\n- bar\n", undefined]], [["", " comment: 2"], [2, undefined], ["b", " this is value comment: b"], ["", undefined]]])).toBe(`\
# comment: 1
- key: 1
  value: a
  options:
    - foo # comment: foo
    - bar
# comment: 2
- key: 2
  value: b # this is value comment: b
`);
    });
});
