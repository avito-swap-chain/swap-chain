import {
  AbsoluteFill,
  Audio,
  interpolate,
  Sequence,
  spring,
  useCurrentFrame,
  useVideoConfig,
} from 'remotion'
import { Caption } from './Caption'
import { Intro } from './Intro'
import { MUSIC } from './Root'
import { Phone, SHOT_HEIGHT, STATUS_BAR_HEIGHT } from './Phone'
import { INTRO_SECONDS, timeline, type Scene } from './scenes'

/** Тот же шрифт и фон, что в приложении: Remotion системный шрифт не наследует. */
export const SURFACE = {
  backgroundColor: '#f4f3f1',
  fontFamily: 'Arial, "Helvetica Neue", Helvetica, sans-serif',
} as const

/**
 * Обычный кадр — подпись сверху и телефон под ней; акцентный — телефон во весь рост.
 * Низ корпуса намеренно уходит за край кадра: интерфейс важнее, чем целый силуэт телефона,
 * а нижняя часть экрана в наших макетах почти всегда пустая.
 */
const SCALE_WITH_CAPTION = 1.32
const SCALE_FOCUSED = 1.62

/**
 * Куда сдвинуть телефон в акцентной сцене, чтобы отмеченная область встала по центру кадра.
 * Без метки просто поднимаем повыше — тогда в кадре остаётся верх экрана.
 */
function focusedTop(highlight: Scene['highlight'], frameHeight: number): number {
  if (!highlight) return 128

  const middle = (highlight.y + highlight.height / 2) * SHOT_HEIGHT * SCALE_FOCUSED
  // Чуть выше геометрического центра: так под областью остаётся видимый контекст.
  return Math.round(frameHeight * 0.46 - middle - STATUS_BAR_HEIGHT * SCALE_FOCUSED)
}

/**
 * Титры, затем один телефон на весь ролик: экраны сменяются внутри него настоящими
 * переходами, а рассказ ведёт подпись сверху. Там, где важнее рассмотреть экран,
 * подпись уходит, телефон вырастает, а кадр наводится на отмеченную область.
 */
export function Demo() {
  const frame = useCurrentFrame()
  const { fps, height } = useVideoConfig()
  const spans = timeline(fps)

  const introFrames = Math.round(INTRO_SECONDS * fps)
  const current = spans.find((span) => frame >= span.from && frame < span.to) ?? spans[0]

  // Масштаб тянется пружиной от границы сцены — телефон не прыгает, а «дышит».
  const focusProgress = spring({
    frame: frame - current.from,
    fps,
    config: { damping: 200 },
    durationInFrames: Math.round(fps * 0.7),
  })
  const previous = spans[spans.indexOf(current) - 1]
  const from = previous?.scene.focus ? SCALE_FOCUSED : SCALE_WITH_CAPTION
  const to = current.scene.focus ? SCALE_FOCUSED : SCALE_WITH_CAPTION
  const scale = interpolate(focusProgress, [0, 1], [from, to])

  // С подписью телефон стоит под текстом. В акцентной сцене кадр наводится на отмеченную
  // область: телефон сдвигается так, чтобы она оказалась по центру, — иначе непонятно,
  // ради чего приближение.
  const topWithCaption = 352
  const topFocused = focusedTop(current.scene.highlight, height)
  const top = interpolate(scale, [SCALE_WITH_CAPTION, SCALE_FOCUSED], [topWithCaption, topFocused])

  const appear = interpolate(frame, [introFrames - fps * 0.4, introFrames + fps * 0.2], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })

  return (
    <AbsoluteFill style={SURFACE}>
      {/* Подложка тихая и однообразная: держит темп, но не спорит с тем, что на экране. */}
      <Audio src={MUSIC} volume={0.55} />

      <Sequence durationInFrames={introFrames}>
        <Intro />
      </Sequence>

      {/* Подпись и телефон живут в абсолютном времени ролика: обернуть их в Sequence нельзя —
          внутри неё отсчёт кадров начинается заново, и сцены разъезжаются с таймлайном. */}
      <Caption spans={spans} />

      <div
        style={{
          position: 'absolute',
          top,
          left: '50%',
          transform: 'translateX(-50%)',
          opacity: appear,
        }}
      >
        <Phone spans={spans} scale={scale} />
      </div>
    </AbsoluteFill>
  )
}
