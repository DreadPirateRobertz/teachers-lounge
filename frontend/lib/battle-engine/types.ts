import * as THREE from 'three'

/** Unique identifier for a loaded asset */
export type AssetId = string

/** Supported asset types for the loader */
export type AssetType = 'gltf' | 'texture' | 'audio'

/** Asset manifest entry describing what to load */
export interface AssetEntry {
  id: AssetId
  type: AssetType
  url: string
}

/** Result of loading a single asset */
export type LoadedAsset =
  | { type: 'gltf'; data: THREE.Group }
  | { type: 'texture'; data: THREE.Texture }
  | { type: 'audio'; data: AudioBuffer }

/** Axis-aligned bounding box for collision detection */
export interface AABB {
  min: THREE.Vector3
  max: THREE.Vector3
}

/** A collidable entity in the scene */
export interface CollisionBody {
  id: string
  bounds: AABB
  /** Layer mask for filtering collisions (bitwise AND) */
  layer: number
  /** Attached scene object, if any */
  object?: THREE.Object3D
}

/** Reported when two bodies overlap */
export interface CollisionEvent {
  bodyA: CollisionBody
  bodyB: CollisionBody
}

/** Callback for the animation loop — receives delta time in seconds */
export type FrameCallback = (dt: number) => void

/** Configuration for the battle scene */
export interface SceneConfig {
  /** Canvas element to render into */
  canvas: HTMLCanvasElement
  /** Enable antialiasing (default: true) */
  antialias?: boolean
  /** Background color (default: 0x0a0a1a — matches bg-deep) */
  backgroundColor?: number
  /** Camera field of view in degrees (default: 60) */
  fov?: number
  /** Pixel ratio cap to prevent GPU overload (default: 2) */
  maxPixelRatio?: number
}
