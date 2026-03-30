import * as THREE from 'three'
import type { AABB, CollisionBody, CollisionEvent } from './types'

/** Collision layers for filtering */
export const CollisionLayer = {
  PLAYER: 1 << 0,
  BOSS: 1 << 1,
  PROJECTILE: 1 << 2,
  POWERUP: 1 << 3,
} as const

/** Creates an AABB from a center point and half-extents */
export function aabb(
  cx: number, cy: number, cz: number,
  hx: number, hy: number, hz: number,
): AABB {
  return {
    min: new THREE.Vector3(cx - hx, cy - hy, cz - hz),
    max: new THREE.Vector3(cx + hx, cy + hy, cz + hz),
  }
}

/** Creates an AABB from a Three.js Object3D's bounding box */
export function aabbFromObject(obj: THREE.Object3D): AABB {
  const box = new THREE.Box3().setFromObject(obj)
  return { min: box.min.clone(), max: box.max.clone() }
}

/** Updates an AABB to match the current world-space bounds of its object */
export function syncBodyToObject(body: CollisionBody): void {
  if (!body.object) return
  const box = new THREE.Box3().setFromObject(body.object)
  body.bounds.min.copy(box.min)
  body.bounds.max.copy(box.max)
}

function aabbOverlap(a: AABB, b: AABB): boolean {
  return (
    a.min.x <= b.max.x && a.max.x >= b.min.x &&
    a.min.y <= b.max.y && a.max.y >= b.min.y &&
    a.min.z <= b.max.z && a.max.z >= b.min.z
  )
}

/**
 * Simple broadphase collision system using AABB overlap tests.
 * Bodies are registered with layer masks for filtering.
 * Call `detect()` each frame to get the current set of collisions.
 */
export function createCollisionSystem() {
  const bodies = new Map<string, CollisionBody>()

  function add(body: CollisionBody) {
    bodies.set(body.id, body)
  }

  function remove(id: string) {
    bodies.delete(id)
  }

  function get(id: string): CollisionBody | undefined {
    return bodies.get(id)
  }

  /** Sync all bodies that have attached objects to their current world positions */
  function syncAll() {
    for (const body of bodies.values()) {
      syncBodyToObject(body)
    }
  }

  /**
   * Run broadphase detection. Returns all overlapping pairs whose
   * layer masks share at least one bit.
   */
  function detect(): CollisionEvent[] {
    const events: CollisionEvent[] = []
    const arr = Array.from(bodies.values())

    for (let i = 0; i < arr.length; i++) {
      for (let j = i + 1; j < arr.length; j++) {
        const a = arr[i]
        const b = arr[j]
        // Skip pairs on the same layer (no self-collision)
        if ((a.layer & b.layer) !== 0) continue
        if (aabbOverlap(a.bounds, b.bounds)) {
          events.push({ bodyA: a, bodyB: b })
        }
      }
    }

    return events
  }

  function clear() {
    bodies.clear()
  }

  function dispose() {
    clear()
  }

  return { add, remove, get, syncAll, detect, clear, dispose }
}

export type CollisionSystem = ReturnType<typeof createCollisionSystem>
