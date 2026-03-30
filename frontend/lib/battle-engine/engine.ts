import type { SceneConfig, AssetEntry, AssetId, LoadedAsset } from './types'
import { createBattleScene, type BattleScene } from './scene'
import { createAssetLoader, type AssetLoader } from './asset-loader'
import { createAnimationLoop, type AnimationLoop } from './animation-loop'
import { createCollisionSystem, type CollisionSystem } from './collision'

/**
 * The top-level battle engine. Orchestrates scene, assets, animation, and collision.
 *
 * Usage:
 *   const engine = createBattleEngine({ canvas })
 *   await engine.loadAssets(manifest, onProgress)
 *   engine.start()
 *   // ... battle logic adds callbacks via engine.loop.add()
 *   engine.dispose()
 */
export function createBattleEngine(config: SceneConfig) {
  const scene: BattleScene = createBattleScene(config)
  const assets: AssetLoader = createAssetLoader()
  const loop: AnimationLoop = createAnimationLoop()
  const collision: CollisionSystem = createCollisionSystem()

  // Register the core render pass and collision sync as loop callbacks
  loop.add('__collision_sync', () => collision.syncAll())
  loop.add('__render', () => scene.render())

  async function loadAssets(
    manifest: AssetEntry[],
    onProgress?: (loaded: number, total: number) => void,
  ): Promise<Map<AssetId, LoadedAsset>> {
    return assets.loadAll(manifest, onProgress)
  }

  function start() {
    loop.start()
  }

  function stop() {
    loop.stop()
  }

  function dispose() {
    loop.dispose()
    collision.dispose()
    assets.dispose()
    scene.dispose()
  }

  return {
    scene,
    assets,
    loop,
    collision,
    loadAssets,
    start,
    stop,
    dispose,
  }
}

export type BattleEngine = ReturnType<typeof createBattleEngine>
