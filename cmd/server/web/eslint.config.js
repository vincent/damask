import ts from '@typescript-eslint/eslint-plugin'
import tsParser from '@typescript-eslint/parser'
import svelte from 'eslint-plugin-svelte'
import svelteParser from 'svelte-eslint-parser'

export default [
  {
    files: ['**/*.ts'],
    plugins: { '@typescript-eslint': ts },
    languageOptions: { parser: tsParser },
    rules: { ...ts.configs.recommended.rules },
  },
  {
    files: ['**/*.svelte'],
    plugins: { svelte },
    languageOptions: {
      parser: svelteParser,
      parserOptions: { parser: tsParser },
    },
    rules: { ...svelte.configs.recommended.rules },
  },
  { ignores: ['.svelte-kit/', 'build/', 'node_modules/'] },
]
