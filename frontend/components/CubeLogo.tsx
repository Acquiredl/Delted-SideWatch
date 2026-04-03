'use client'

/** A small 3D CSS Rubik's cube that rotates. Used in the navigation bar. */
export default function CubeLogo() {
  return (
    <div className="cube-scene" aria-hidden="true">
      <div className="cube-body">
        <div className="cube-face cube-front" />
        <div className="cube-face cube-back" />
        <div className="cube-face cube-right" />
        <div className="cube-face cube-left" />
        <div className="cube-face cube-top" />
        <div className="cube-face cube-bottom" />
      </div>
    </div>
  )
}
