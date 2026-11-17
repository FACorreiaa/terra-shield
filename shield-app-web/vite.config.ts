import { defineConfig } from "vite-plus";

import { TimeShardSequencer } from "./tools/ci/src/sequencer.ts";

const generated = ["**/dist/**", "**/*.gen.*", "**/routeTree.gen.ts", "**/.ci/**"];
const testFiles = ["**/*.test.{ts,tsx}"];

export default defineConfig({
  fmt: {
    printWidth: 120,
    ignorePatterns: generated,
    sortImports: true,
    sortTailwindcss: { stylesheet: "apps/web/src/styles/app.css", functions: ["cn", "cva", "clsx"] },
  },
  lint: {
    jsPlugins: [{ name: "vite-plus", specifier: "vite-plus/oxlint-plugin" }],
    // Every plugin that applies to this stack and every stable category at error. Nursery rules
    // are unstable upstream and stay off. A rule below is off or tuned only when it contradicts
    // another enabled rule or the formatter, does not apply to TypeScript / React 19 / ES2023, or
    // cannot be satisfied with third-party types. Each says which.
    plugins: [
      "eslint",
      "typescript",
      "unicorn",
      "oxc",
      "import",
      "react",
      "react-perf",
      "jsx-a11y",
      "promise",
      "jsdoc",
    ],
    categories: {
      correctness: "error",
      suspicious: "error",
      pedantic: "error",
      perf: "error",
      style: "error",
      restriction: "error",
    },
    rules: {
      "vite-plus/prefer-vite-plus-imports": "error",

      // Contradictions: the first rule of each pair is off, the second stays on.
      // import/no-named-export and import/prefer-default-export vs import/no-default-export.
      "import/no-named-export": "off",
      "import/prefer-default-export": "off",
      // eslint/no-undefined vs unicorn/no-null: together they leave no way to say "absent".
      "eslint/no-undefined": "off",
      // eslint/sort-imports vs the formatter's sortImports, which owns import order.
      "eslint/sort-imports": "off",
      // typescript/promise-function-async vs typescript/require-await: a function that implements
      // a Promise-returning contract without awaiting (a fetch stub, a sequencer hook) can't satisfy both.
      "typescript/promise-function-async": "off",
      // typescript/no-floating-promises asks for `void promise`; eslint/no-void allows exactly that.
      "eslint/no-void": ["error", { allowAsStatement: true }],
      // Type-only imports stay top-level (typescript/no-import-type-side-effects), so a module can
      // be imported twice: once for types, once for values.
      "import/consistent-type-specifier-style": ["error", "prefer-top-level"],
      "eslint/no-duplicate-imports": ["error", { allowSeparateTypeImports: true }],

      // Not applicable to this stack.
      // The automatic JSX runtime (React 17+) needs no React in scope.
      "react/react-in-jsx-scope": "off",
      // These three restrict syntax for pre-ES2017/2018/2020 targets; we target ES2023.
      "oxc/no-optional-chaining": "off",
      "oxc/no-async-await": "off",
      "oxc/no-rest-spread-properties": "off",
      // TypeScript signatures carry parameter and return types.
      "jsdoc/require-param": "off",
      "jsdoc/require-returns": "off",
      // No i18n layer yet; switch on with one.
      "react/jsx-no-literals": "off",
      // Key order carries meaning: GeoJSON, configs, CVA variants.
      "eslint/sort-keys": "off",
      // The conditional-render idiom; eslint/no-nested-ternary stays on.
      "eslint/no-ternary": "off",
      // className passthrough is the shadcn/ui contract.
      "react/forbid-component-props": "off",

      // Contradicts typescript/consistent-return, which wants the explicit `return undefined`.
      "unicorn/no-useless-undefined": "off",
      // Reads every wrapped line of a multi-line `//` comment as a new sentence.
      "eslint/capitalized-comments": "off",

      // Cannot be satisfied: React, MapLibre and Vitest types are not deeply readonly.
      "typescript/prefer-readonly-parameter-types": "off",

      // Tuned, still on.
      "react/jsx-filename-extension": ["error", { extensions: [".tsx"] }],
      "eslint/one-var": ["error", "never"],
      "eslint/func-style": ["error", "declaration", { allowArrowFunctions: true }],
      "eslint/no-magic-numbers": [
        "error",
        {
          ignore: [-1, 0, 1, 2],
          ignoreArrayIndexes: true,
          ignoreDefaultValues: true,
          ignoreClassFieldInitialValues: true,
        },
      ],
      "eslint/max-statements": ["error", { max: 15 }],
      "eslint/max-lines-per-function": ["error", { max: 80, skipBlankLines: true, skipComments: true }],
      // openapi-fetch's methods are named after HTTP verbs.
      "eslint/new-cap": ["error", { capIsNewExceptions: ["GET", "POST", "PUT", "PATCH", "DELETE"] }],
      // Callbacks passed straight to a typed parameter take its type.
      "typescript/explicit-function-return-type": ["error", { allowExpressions: true }],
      "react/jsx-max-depth": ["error", { max: 6 }],
      // TanStack Router file routes export `Route` next to the component.
      "react/only-export-components": ["error", { allowExportNames: ["Route"] }],
      "unicorn/filename-case": [
        "error",
        { cases: { kebabCase: true, camelCase: true, pascalCase: true }, ignore: [String.raw`^__root\.tsx$`] },
      ],
      "import/no-unassigned-import": ["error", { allow: ["**/*.css"] }],
      // A module may use a type and re-export it; only pure pass-throughs must be `export from`.
      "unicorn/prefer-export-from": ["error", { checkUsedVariables: false }],
      // A file may keep small presentational helpers next to the component it exports.
      "react/no-multi-comp": ["error", { ignoreStateless: true }],
    },
    overrides: [
      {
        // Config files and Vitest reporters must default-export; tools run in Node.
        files: ["**/*.config.ts", "tools/**"],
        rules: {
          "import/no-default-export": "off",
          "import/no-nodejs-modules": "off",
          "node/no-process-env": "off",
          "node/no-sync": "off",
        },
        plugins: ["node"],
      },
      {
        // TanStack Router file routes pass their component to `createFileRoute` and export only
        // `Route`, which the rule reads as a file that exports no component.
        files: ["apps/web/src/routes/**"],
        rules: { "react/only-export-components": "off" },
      },
      {
        // shadcn/ui primitives forward every prop by design.
        files: ["apps/web/src/components/ui/**"],
        rules: { "react/jsx-props-no-spreading": "off", "react/button-has-type": "off" },
      },
      {
        // Vitest rules apply to Vitest files only.
        files: testFiles,
        plugins: ["vitest"],
        rules: {
          // The rule set's other side, prefer-importing-vitest-globals, stays on; and require-hook
          // stays on against no-hooks.
          "vitest/no-importing-vitest-globals": "off",
          "vitest/no-hooks": "off",
          // The global testTimeout applies to every test.
          "vitest/require-test-timeout": "off",
          // Contradict vitest/prefer-strict-boolean-matchers, which stays on.
          "vitest/prefer-to-be-truthy": "off",
          "vitest/prefer-to-be-falsy": "off",
          // prefer-describe-function-title passes a function as the title.
          "vitest/valid-title": ["error", { ignoreTypeOfDescribeName: true }],
          "vitest/prefer-expect-assertions": ["error", { onlyFunctionsWithAsyncKeyword: true }],
        },
      },
      {
        // Tests and e2e specs: data literals and long blocks are the point.
        files: [...testFiles, "**/e2e/**"],
        rules: {
          "eslint/no-magic-numbers": "off",
          "eslint/max-lines-per-function": "off",
          "eslint/max-statements": "off",
        },
      },
    ],
    ignorePatterns: generated,
    options: { typeAware: true, typeCheck: true },
  },
  test: {
    projects: ["apps/*", "packages/*", "tools/*"],
    // `--shard i/n` splits by recorded time instead of hashed path (tools/ci).
    sequence: { sequencer: TimeShardSequencer },
    // Writes per-file times when TEST_TIMINGS_OUT is set (CI); a no-op otherwise.
    reporters: ["default", "./tools/ci/src/timingsReporter.ts"],
    // `--changed` follows the import graph from the diff; these files reach every test.
    forceRerunTriggers: [
      "**/package.json/**",
      "**/bun.lock/**",
      "**/{vite,vitest}.config.*/**",
      "**/tsconfig*.json/**",
      "tools/ci/**",
    ],
    passWithNoTests: true,
  },
  run: {
    cache: true,
  },
});
