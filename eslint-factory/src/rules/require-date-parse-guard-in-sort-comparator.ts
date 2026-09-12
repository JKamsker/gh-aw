import { AST_NODE_TYPES, ESLintUtils, TSESTree } from "@typescript-eslint/utils";

const createRule = ESLintUtils.RuleCreator(name => `https://github.com/github/gh-aw/tree/main/eslint-factory#${name}`);

/**
 * Returns true when the call expression is `Date.parse(x)` (an argument is required; a bare
 * `Date.parse()` is not a realistic pattern worth special-casing away).
 */
function isDateParseCall(node: TSESTree.Node): boolean {
  if (node.type !== AST_NODE_TYPES.CallExpression) return false;
  if (node.callee.type !== AST_NODE_TYPES.MemberExpression) return false;
  const { object, property, computed } = node.callee;
  if (computed || object.type !== AST_NODE_TYPES.Identifier || object.name !== "Date") return false;
  if (property.type !== AST_NODE_TYPES.Identifier || property.name !== "parse") return false;
  return node.arguments.length >= 1;
}

/**
 * Returns true when `node`'s parent is a `.sort(comparator)` call and `node` is the comparator
 * argument passed to it (directly, not via a named reference the rule cannot statically follow).
 */
function isSortComparator(node: TSESTree.ArrowFunctionExpression | TSESTree.FunctionExpression): boolean {
  const parent = node.parent;
  if (!parent || parent.type !== AST_NODE_TYPES.CallExpression) return false;
  if (parent.arguments[0] !== node) return false;
  const callee = parent.callee;
  if (callee.type !== AST_NODE_TYPES.MemberExpression || callee.computed) return false;
  const prop = callee.property;
  return prop.type === AST_NODE_TYPES.Identifier && prop.name === "sort";
}

/**
 * Walks up from a `BinaryExpression` through `ReturnStatement`/`BlockStatement` wrappers to find
 * the enclosing function, returning it only when the expression is the value actually returned
 * by that function (an arrow's expression body, or a `return <expr>;` in a block body).
 */
function findEnclosingComparatorFunction(node: TSESTree.BinaryExpression): TSESTree.ArrowFunctionExpression | TSESTree.FunctionExpression | null {
  const parent = node.parent;
  if (!parent) return null;

  // Arrow function with an expression body: `(a, b) => Date.parse(a) - Date.parse(b)`.
  if ((parent.type === AST_NODE_TYPES.ArrowFunctionExpression || parent.type === AST_NODE_TYPES.FunctionExpression) && parent.body === node) {
    return parent;
  }

  // `return Date.parse(a) - Date.parse(b);` as the sole statement of a block body.
  if (parent.type === AST_NODE_TYPES.ReturnStatement) {
    const block = parent.parent;
    if (block?.type === AST_NODE_TYPES.BlockStatement && (block.parent?.type === AST_NODE_TYPES.ArrowFunctionExpression || block.parent?.type === AST_NODE_TYPES.FunctionExpression)) {
      return block.parent;
    }
  }

  return null;
}

export const requireDateParseGuardInSortComparatorRule = createRule({
  name: "require-date-parse-guard-in-sort-comparator",
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow `Date.parse(x) - Date.parse(y)` as the return value of a `.sort()` comparator. " +
        "`Date.parse` returns NaN for an unparseable string, and `Array.prototype.sort` treats a NaN comparator result as 0 (elements compared equal), " +
        "so a single malformed date silently produces an arbitrary, non-chronological order instead of surfacing a parse error.",
    },
    schema: [],
    messages: {
      requireGuard:
        "This `.sort()` comparator returns `Date.parse(...) - Date.parse(...)`. If either side fails to parse, Date.parse returns NaN, and `Array.prototype.sort` treats a NaN result as 0 — silently producing an arbitrary, non-chronological order instead of surfacing the invalid date. Validate both timestamps (e.g. with Number.isFinite) before sorting, or pre-filter/normalize entries with unparseable dates.",
    },
  },
  defaultOptions: [],
  create(context) {
    return {
      BinaryExpression(node) {
        if (node.operator !== "-") return;
        if (!isDateParseCall(node.left) || !isDateParseCall(node.right)) return;

        const comparator = findEnclosingComparatorFunction(node);
        if (comparator === null || !isSortComparator(comparator)) return;

        context.report({ node, messageId: "requireGuard" });
      },
    };
  },
});
