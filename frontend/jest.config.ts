import type { Config } from 'jest'

const config: Config = {
  testEnvironment: 'node',
  transform: {
    '^.+\\.tsx?$': ['ts-jest', { tsconfig: { moduleResolution: 'node', jsx: 'react-jsx' } }],
  },
  moduleNameMapper: {
    '^@/(.*)$': '<rootDir>/$1',
    // Stub CSS imports (katex, etc.) so they don't crash in jsdom
    '\\.css$': '<rootDir>/__mocks__/styleMock.js',
    // Stub next/dynamic to render components synchronously in tests
    '^next/dynamic$': '<rootDir>/__mocks__/nextDynamic.js',
  },
  testMatch: ['**/*.test.ts', '**/*.test.tsx'],
  setupFilesAfterEnv: ['@testing-library/jest-dom'],
}

export default config
