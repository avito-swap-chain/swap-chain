import { Easing, interpolate, useCurrentFrame, useVideoConfig } from 'remotion'
import type { Span } from './scenes'

/**
 * Свечение по краям экрана, пока работает распознавание: цветная кайма медленно вращается
 * и растворяется к центру. Рисуется поверх всего экрана телефона, включая строку состояния.
 */
export function AiGlow({ spans, radius }: { spans: Span[]; radius: number }) {
  const frame = useCurrentFrame()
  const { fps } = useVideoConfig()

  const active = spans.find((span) => {
    const window = span.scene.aiGlow
    if (!window) return false
    return frame >= span.from + window.from * fps && frame <= span.from + window.to * fps
  })
  if (!active?.scene.aiGlow) return null

  const from = active.from + active.scene.aiGlow.from * fps
  const to = active.from + active.scene.aiGlow.to * fps

  // Разгон и затухание не длиннее двух пятых окна: на коротком окне они иначе налезают
  // друг на друга, и точки интерполяции перестают идти по возрастанию.
  const duration = to - from
  const rise = Math.min(fps * 0.35, duration * 0.4)
  const fall = Math.min(fps * 0.5, duration * 0.4)
  const alive = interpolate(frame, [from, from + rise, to - fall, to], [0, 1, 1, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
    easing: Easing.inOut(Easing.quad),
  })
  if (alive <= 0) return null

  const angle = ((frame - from) / fps) * 80
  const breath = (Math.sin(((frame - from) / fps) * Math.PI * 1.6) + 1) / 2
  const depth = interpolate(breath, [0, 1], [58, 78])

  const rainbow = `conic-gradient(from ${angle}deg, #00aaff, #965eeb, #ff4053, #ffa200, #04e061, #00aaff)`
  // Эллипс меньше экрана: внутри него цвет снят, к краям набирается полностью,
  // за пределами — сплошной. Так кайма идёт по всему периметру и тает к середине.
  const fade = `radial-gradient(82% 60% at 50% 50%, transparent 58%, rgba(0,0,0,0.45) 86%, #000 100%)`

  return (
    <div
      style={{
        position: 'absolute',
        inset: 0,
        borderRadius: radius,
        overflow: 'hidden',
        pointerEvents: 'none',
        // Выше экранов: у каждого из них свой z-index в стопке переходов.
        zIndex: 50,
        // Подложка, а не заливка: интерфейс под свечением должен читаться.
        opacity: alive * 0.62,
      }}
    >
      <div
        style={{
          position: 'absolute',
          inset: 0,
          background: rainbow,
          filter: `blur(${depth * 0.34}px)`,
          maskImage: fade,
          WebkitMaskImage: fade,
        }}
      />
    </div>
  )
}
