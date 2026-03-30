import * as THREE from 'three'
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js'
import type { AssetEntry, AssetId, LoadedAsset } from './types'

/**
 * Loads and caches battle assets (GLTF models, textures, audio).
 * Assets are loaded once and reused across battles.
 * Call `dispose()` to release all cached GPU/memory resources.
 */
export function createAssetLoader() {
  const cache = new Map<AssetId, LoadedAsset>()
  const gltfLoader = new GLTFLoader()
  const textureLoader = new THREE.TextureLoader()

  function get(id: AssetId): LoadedAsset | undefined {
    return cache.get(id)
  }

  async function load(entry: AssetEntry): Promise<LoadedAsset> {
    const cached = cache.get(entry.id)
    if (cached) return cached

    let asset: LoadedAsset

    switch (entry.type) {
      case 'gltf': {
        const gltf = await gltfLoader.loadAsync(entry.url)
        asset = { type: 'gltf', data: gltf.scene }
        break
      }
      case 'texture': {
        const tex = await textureLoader.loadAsync(entry.url)
        tex.colorSpace = THREE.SRGBColorSpace
        asset = { type: 'texture', data: tex }
        break
      }
      case 'audio': {
        const resp = await fetch(entry.url)
        const buf = await resp.arrayBuffer()
        const ctx = new AudioContext()
        const decoded = await ctx.decodeAudioData(buf)
        asset = { type: 'audio', data: decoded }
        break
      }
    }

    cache.set(entry.id, asset)
    return asset
  }

  async function loadAll(
    entries: AssetEntry[],
    onProgress?: (loaded: number, total: number) => void,
  ): Promise<Map<AssetId, LoadedAsset>> {
    const results = new Map<AssetId, LoadedAsset>()
    let loaded = 0

    // Load in parallel
    await Promise.all(
      entries.map(async (entry) => {
        const asset = await load(entry)
        results.set(entry.id, asset)
        loaded++
        onProgress?.(loaded, entries.length)
      }),
    )

    return results
  }

  function dispose() {
    for (const [, asset] of cache) {
      switch (asset.type) {
        case 'gltf':
          asset.data.traverse((obj) => {
            if (obj instanceof THREE.Mesh) {
              obj.geometry.dispose()
              const mat = obj.material
              if (Array.isArray(mat)) mat.forEach((m) => m.dispose())
              else mat.dispose()
            }
          })
          break
        case 'texture':
          asset.data.dispose()
          break
        case 'audio':
          // AudioBuffer is GC'd automatically
          break
      }
    }
    cache.clear()
  }

  return { get, load, loadAll, dispose }
}

export type AssetLoader = ReturnType<typeof createAssetLoader>
