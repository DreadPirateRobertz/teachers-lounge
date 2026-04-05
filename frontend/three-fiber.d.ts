/**
 * Extends React JSX intrinsic elements with @react-three/fiber's Three.js types.
 * Required so that Three.js mesh/material/geometry tags are valid JSX.
 */
import { ThreeElements } from '@react-three/fiber'

declare module 'react' {
  namespace JSX {
    interface IntrinsicElements extends ThreeElements {}
  }
}
