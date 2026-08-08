import { Easing, interpolate, useCurrentFrame, useVideoConfig } from 'remotion'
import type { Span } from './scenes'

const INK = '#0a0a0a'
const INK_2 = '#575759'

/** Подпись сменяется чуть раньше экрана — сначала объяснение, потом то, что оно объясняет. */
const SWAP_SECONDS = 0.3

/**
 * Подпись над телефоном. Живёт отдельным слоем и меняется по ходу сценария,
 * а в акцентных сценах уходит совсем — тогда весь кадр отдан экрану.
 */
export function Caption({ spans }: { spans: Span[] }) {
  const frame = useCurrentFrame()
  const { fps, width } = useVideoConfig()

  const swap = Math.round(SWAP_SECONDS * fps)
  const current = spans.find((span) => frame >= span.from - swap && frame < span.to - swap)
  if (!current) return null

  const { scene } = current
  // Акцентная сцена идёт без подписи: кадр целиком отдан экрану.
  if (scene.focus) return null

  const appear = interpolate(frame, [current.from - swap, current.from], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
    easing: Easing.out(Easing.cubic),
  })
  const hide = interpolate(frame, [current.to - swap * 2, current.to - swap], [1, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })

  const opacity = appear * hide
  const lift = interpolate(appear, [0, 1], [16, 0])

  return (
    <div
      style={{
        position: 'absolute',
        top: 96,
        left: '50%',
        transform: `translate(-50%, ${lift}px)`,
        width: width - 150,
        textAlign: 'center',
        opacity,
      }}
    >
      <h1
        style={{
          margin: 0,
          fontSize: 58,
          lineHeight: 1.08,
          color: INK,
          letterSpacing: -1.5,
        }}
      >
        {scene.title}
      </h1>
      <p
        style={{
          margin: '18px 0 0',
          fontSize: 29,
          lineHeight: 1.35,
          color: INK_2,
        }}
      >
        {scene.caption}
      </p>
    </div>
  )
}
