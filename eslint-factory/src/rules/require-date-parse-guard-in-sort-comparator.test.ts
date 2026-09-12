import { RuleTester } from "eslint";
import { describe, it } from "vitest";
import { requireDateParseGuardInSortComparatorRule } from "./require-date-parse-guard-in-sort-comparator";

const cjsRuleTester = new RuleTester({
  languageOptions: {
    ecmaVersion: 2022,
    sourceType: "commonjs",
  },
});

describe("require-date-parse-guard-in-sort-comparator", () => {
  it("invalid: arrow-body comparator subtracting Date.parse(...) results", () => {
    cjsRuleTester.run("require-date-parse-guard-in-sort-comparator", requireDateParseGuardInSortComparatorRule, {
      valid: [],
      invalid: [
        {
          code: `runs.sort((a, b) => Date.parse(b.created_at || "") - Date.parse(a.created_at || ""));`,
          errors: [{ messageId: "requireGuard" }],
        },
        {
          code: `reviews.sort((a, b) => Date.parse(a.submitted_at) - Date.parse(b.submitted_at));`,
          errors: [{ messageId: "requireGuard" }],
        },
      ],
    });
  });

  it("invalid: block-body comparator with an explicit return statement", () => {
    cjsRuleTester.run("require-date-parse-guard-in-sort-comparator", requireDateParseGuardInSortComparatorRule, {
      valid: [],
      invalid: [
        {
          code: `items.sort(function (a, b) { return Date.parse(a.updatedAt) - Date.parse(b.updatedAt); });`,
          errors: [{ messageId: "requireGuard" }],
        },
      ],
    });
  });

  it("valid: comparator subtracting plain numeric fields is not flagged", () => {
    cjsRuleTester.run("require-date-parse-guard-in-sort-comparator", requireDateParseGuardInSortComparatorRule, {
      valid: [`runs.sort((a, b) => b.mtimeMs - a.mtimeMs);`, `runs.sort((a, b) => a.time.getTime() - b.time.getTime());`, `runs.sort((a, b) => Number(b.id) - Number(a.id));`],
      invalid: [],
    });
  });

  it("valid: Date.parse subtraction outside a .sort() comparator is not flagged", () => {
    cjsRuleTester.run("require-date-parse-guard-in-sort-comparator", requireDateParseGuardInSortComparatorRule, {
      valid: [`const diffMs = Date.parse(b.updated_at) - Date.parse(a.updated_at);`, `const values = arr.map((a, b) => Date.parse(a.x) - Date.parse(b.x));`],
      invalid: [],
    });
  });

  it("valid: Date.parse subtraction guarded and returned via a named comparator function is not flagged (not statically resolvable)", () => {
    cjsRuleTester.run("require-date-parse-guard-in-sort-comparator", requireDateParseGuardInSortComparatorRule, {
      valid: [
        `function byDate(a, b) { const av = Date.parse(a.d); const bv = Date.parse(b.d); if (!Number.isFinite(av) || !Number.isFinite(bv)) return 0; return av - bv; } runs.sort(byDate);`,
      ],
      invalid: [],
    });
  });

  it("valid: only one side is Date.parse (mixed shape) is not flagged", () => {
    cjsRuleTester.run("require-date-parse-guard-in-sort-comparator", requireDateParseGuardInSortComparatorRule, {
      valid: [`runs.sort((a, b) => Date.parse(a.d) - b.timestamp);`],
      invalid: [],
    });
  });
});
