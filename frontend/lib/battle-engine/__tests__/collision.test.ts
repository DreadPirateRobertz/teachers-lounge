import { createCollisionSystem, CollisionLayer, aabb } from '../collision'

describe('createCollisionSystem', () => {
  it('detects overlapping bodies on different layers', () => {
    const system = createCollisionSystem()

    system.add({
      id: 'player',
      bounds: aabb(0, 0, 0, 1, 1, 1),
      layer: CollisionLayer.PLAYER,
    })
    system.add({
      id: 'boss',
      bounds: aabb(0.5, 0, 0, 1, 1, 1),
      layer: CollisionLayer.BOSS,
    })

    const events = system.detect()
    expect(events).toHaveLength(1)
    expect(events[0].bodyA.id).toBe('player')
    expect(events[0].bodyB.id).toBe('boss')

    system.dispose()
  })

  it('ignores bodies on the same layer', () => {
    const system = createCollisionSystem()

    system.add({
      id: 'proj1',
      bounds: aabb(0, 0, 0, 1, 1, 1),
      layer: CollisionLayer.PROJECTILE,
    })
    system.add({
      id: 'proj2',
      bounds: aabb(0.5, 0, 0, 1, 1, 1),
      layer: CollisionLayer.PROJECTILE,
    })

    const events = system.detect()
    expect(events).toHaveLength(0)

    system.dispose()
  })

  it('ignores non-overlapping bodies', () => {
    const system = createCollisionSystem()

    system.add({
      id: 'player',
      bounds: aabb(0, 0, 0, 0.5, 0.5, 0.5),
      layer: CollisionLayer.PLAYER,
    })
    system.add({
      id: 'boss',
      bounds: aabb(10, 10, 10, 0.5, 0.5, 0.5),
      layer: CollisionLayer.BOSS,
    })

    const events = system.detect()
    expect(events).toHaveLength(0)

    system.dispose()
  })

  it('removes bodies by id', () => {
    const system = createCollisionSystem()

    system.add({
      id: 'a',
      bounds: aabb(0, 0, 0, 1, 1, 1),
      layer: CollisionLayer.PLAYER,
    })
    system.add({
      id: 'b',
      bounds: aabb(0, 0, 0, 1, 1, 1),
      layer: CollisionLayer.BOSS,
    })

    system.remove('a')
    const events = system.detect()
    expect(events).toHaveLength(0)

    system.dispose()
  })

  it('clears all bodies', () => {
    const system = createCollisionSystem()

    system.add({
      id: 'a',
      bounds: aabb(0, 0, 0, 1, 1, 1),
      layer: CollisionLayer.PLAYER,
    })
    system.add({
      id: 'b',
      bounds: aabb(0, 0, 0, 1, 1, 1),
      layer: CollisionLayer.BOSS,
    })

    system.clear()
    const events = system.detect()
    expect(events).toHaveLength(0)

    system.dispose()
  })
})

describe('aabb', () => {
  it('creates correct min/max from center + half-extents', () => {
    const box = aabb(1, 2, 3, 0.5, 1, 1.5)
    expect(box.min.x).toBe(0.5)
    expect(box.min.y).toBe(1)
    expect(box.min.z).toBe(1.5)
    expect(box.max.x).toBe(1.5)
    expect(box.max.y).toBe(3)
    expect(box.max.z).toBe(4.5)
  })
})
