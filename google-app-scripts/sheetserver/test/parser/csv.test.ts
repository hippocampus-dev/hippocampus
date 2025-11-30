import {stringifyRFC4180} from "../../src/parser/csv";
import {describe, expect, test} from "vitest";

describe("stringifyRFC4180", () => {
    test("simple", () => {
        expect(stringifyRFC4180([["a", "b", "c"], ["1", "2", "3"], ["4", "5", "6"], ["7", "8", "9"]])).toBe(`\
a,b,c
1,2,3
4,5,6
7,8,9
`);
    });

    test("with empty fields", () => {
        expect(stringifyRFC4180([["a", "b", "c"], ["1", "", "3"], ["4", "5", ""], ["", "8", "9"]])).toBe(`\
a,b,c
1,,3
4,5,
,8,9
`);
    });

    test("with double quotes", () => {
        expect(stringifyRFC4180([["a", "b", "c"], ["1", "2", "3"], ["4", "5", "6"], ["7", "8", "9"]])).toBe(`\
a,b,c
1,2,3
4,5,6
7,8,9
`);
    });

    test("with double quotes and empty fields", () => {
        expect(stringifyRFC4180([["a", "b", "c"], ["1", "", "3"], ["4", "5", ""], ["", "8", "9"]])).toBe(`\
a,b,c
1,,3
4,5,
,8,9
`);
    });

    test("with double quotes and double quotes", () => {
        expect(stringifyRFC4180([["a", "b", "c"], ["1", "2", "3"], ["4", '"5"', "6"], ["7", "8", "9"]])).toBe(`\
a,b,c
1,2,3
4,"""5""",6
7,8,9
`);
    });

    test("with multiline", () => {
        expect(stringifyRFC4180([["a", "b", "c"], ["1", "2", "3"], ["4", "5\n5", "6"], ["7", "8", "9"]])).toBe(`\
a,b,c
1,2,3
4,"5\n5",6
7,8,9
`);
    });

    test("with comma in field", () => {
        expect(stringifyRFC4180([["a", "b", "c"], ["1", "2", "3"], ["4", "5,5", "6"], ["7", "8", "9"]])).toBe(`\
a,b,c
1,2,3
4,"5,5",6
7,8,9
`);
    });
});
