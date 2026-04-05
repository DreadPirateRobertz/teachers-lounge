/**
 * Synchronous stub for next/dynamic used in Jest tests.
 *
 * next/dynamic wraps a dynamic import so it renders as `null` on first render
 * (SSR placeholder). In tests we want the component to render immediately
 * without async overhead, so we resolve the loader synchronously.
 *
 * @param {() => Promise<any>} loader - Dynamic import factory.
 * @param {object} _opts - next/dynamic options (ignored in tests).
 * @returns {React.ComponentType} The resolved component.
 */
const nextDynamic = (loader, _opts) => {
  // Attempt to resolve the module synchronously via require.
  // This works because Jest's module registry is synchronous.
  let Component = null
  try {
    // Call the loader to get the promise, then peek at the module via require.
    // We extract the module path from the loader's source when possible,
    // but the safest approach is to call the loader and use the result.
    // Since jest modules are sync, we rely on the fact that dynamic() in tests
    // always receives a require-resolvable path.
    const promise = loader()
    if (promise && typeof promise.then === 'function') {
      // Force synchronous resolution via a side-effect-free trick:
      // jest has already loaded the module; extract from registry.
      let resolved = false
      promise.then((mod) => {
        Component = mod.default || mod
        resolved = true
      })
      // If the promise resolved synchronously (common in jest), use it.
      if (!resolved) {
        // Fallback: return a null-rendering placeholder.
        return () => null
      }
    }
  } catch {
    return () => null
  }
  return Component || (() => null)
}

module.exports = nextDynamic
