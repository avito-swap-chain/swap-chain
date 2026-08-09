import { useEffect, useState } from 'react'
import { getImageDimensions } from '@remotion/media-utils'
import {
  continueRender,
  delayRender,
  Easing,
  Img,
  interpolate,
  staticFile,
  useCurrentFrame,
  useVideoConfig,
} from 'remotion'
import { AiGlow } from './AiGlow'
import type { Span } from './scenes'

/** Экран снимаем при 430×800 — те же пропорции держим в кадре. */
export const SHOT_WIDTH = 560
export const SHOT_HEIGHT = Math.round((800 * SHOT_WIDTH) / 430)

/** Строка состояния — часть корпуса, а не наложение поверх: контент под ней не теряется. */
export const STATUS_BAR_HEIGHT = 44

/** Длительность перехода между экранами: примерно столько же длится переход в приложении. */
const TRANSITION_SECONDS = 0.42

/** Реальные размеры файла: нужны, чтобы понять, насколько экран длиннее окна. */
function useImageSize(src: string) {
  const [size, setSize] = useState<{ width: number; height: number }>()

  useEffect(() => {
    const handle = delayRender(`размеры ${src}`)
    getImageDimensions(src)
      .then(setSize)
      .finally(() => continueRender(handle))
  }, [src])

  return size
}

/**
 * Корпус телефона. Он один на весь ролик и не меняет положения — двигается только
 * содержимое, как на настоящем устройстве. Масштаб задаёт кадр: в акцентных сценах
 * телефон вырастает, когда подпись уходит.
 */
export function Phone({ spans, scale }: { spans: Span[]; scale: number }) {
  return (
    <div
      style={{
        position: 'relative',
        transform: `scale(${scale})`,
        transformOrigin: 'center top',
        borderRadius: 68,
        padding: 14,
        backgroundColor: '#1c1c1e',
        boxShadow:
          '0 40px 80px -30px rgba(10, 30, 50, 0.45), inset 0 0 0 2px rgba(255, 255, 255, 0.14)',
      }}
    >
      <div
        style={{
          position: 'relative',
          overflow: 'hidden',
          borderRadius: 54,
          backgroundColor: '#fff',
        }}
      >
        <StatusBar />

        <div
          style={{
            position: 'relative',
            width: SHOT_WIDTH,
            height: SHOT_HEIGHT,
            overflow: 'hidden',
          }}
        >
          {spans.map((span, index) => (
            <PhoneScreen key={span.scene.image} span={span} index={index} />
          ))}
        </div>

        <AiGlow spans={spans} radius={54} />
      </div>

      {/* Кнопки громкости на торце достраивают силуэт. Островка камеры нет намеренно:
          он ложится поверх шапки приложения и съедает как раз ту строку, ради которой всё снято. */}
      <SideButton top={150} />
      <SideButton top={262} />
    </div>
  )
}

/**
 * Один экран внутри телефона. Появляется своим переходом, а когда наступает очередь
 * следующего — уходит его же обратным движением: экраны не мигают друг сквозь друга.
 */
function PhoneScreen({ span, index }: { span: Span; index: number }) {
  const frame = useCurrentFrame()
  const { fps } = useVideoConfig()

  const src = staticFile(`screens/${span.scene.image}`)
  const size = useImageSize(src)

  const transition = Math.round(TRANSITION_SECONDS * fps)
  const kind = span.scene.transition ?? 'fade'

  // Экран живёт от начала своего появления до момента, когда его сменил следующий.
  const visibleFrom = span.from - transition
  if (frame < visibleFrom || frame >= span.to) return null

  const easing = {
    easing: Easing.out(Easing.cubic),
    extrapolateLeft: 'clamp' as const,
    extrapolateRight: 'clamp' as const,
  }
  const enter = interpolate(frame, [visibleFrom, span.from], [0, 1], easing)
  const leave = interpolate(frame, [span.to - transition, span.to], [0, 1], easing)

  const slideIn = kind === 'push' ? SHOT_WIDTH : kind === 'back' ? -SHOT_WIDTH : 0
  // Уходящий экран сдвигается на треть — так в iOS читается глубина, а не подмена картинки.
  const slideOut = kind === 'push' ? -SHOT_WIDTH * 0.32 : kind === 'back' ? SHOT_WIDTH * 0.32 : 0

  const x = interpolate(enter, [0, 1], [slideIn, 0]) + interpolate(leave, [0, 1], [0, slideOut])
  const opacity = kind === 'fade' ? enter * (1 - leave) : 1 - leave * 0.35

  // Экран длиннее окна — прокручиваем его, как пальцем: короткие остаются неподвижными.
  const shownHeight = size ? (size.height * SHOT_WIDTH) / size.width : SHOT_HEIGHT
  const overflow = Math.max(0, shownHeight - SHOT_HEIGHT)
  const scroll = interpolate(frame, [span.from + fps * 0.8, span.to - fps * 0.5], [0, -overflow], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
    easing: Easing.inOut(Easing.cubic),
  })

  return (
    <div
      style={{
        position: 'absolute',
        inset: 0,
        transform: `translateX(${x}px)`,
        opacity,
        backgroundColor: '#fff',
        // Позже по сценарию — выше в стопке: входящий экран всегда поверх уходящего.
        zIndex: index,
      }}
    >
      <Img
        src={src}
        style={{
          display: 'block',
          width: SHOT_WIDTH,
          transform: `translateY(${scroll}px)`,
        }}
      />

      <Highlight span={span} />
      <Tap span={span} />
    </div>
  )
}

/**
 * Пульсирующая обводка вокруг области акцента. Приближение само по себе ничего не объясняет —
 * метка показывает, на что именно смотреть, пока подписи нет.
 */
function Highlight({ span }: { span: Span }) {
  const frame = useCurrentFrame()
  const { fps } = useVideoConfig()

  const area = span.scene.highlight
  if (!area) return null

  // Появляется не сразу: сначала экран встаёт на место, потом на нём загорается метка.
  const appear = interpolate(frame, [span.from + fps * 0.35, span.from + fps * 0.9], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
    easing: Easing.out(Easing.cubic),
  })
  const hide = interpolate(frame, [span.to - fps * 0.5, span.to - fps * 0.2], [1, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })

  // Дыхание раз в полторы секунды: заметно, но не мигает.
  const pulse = (Math.sin(((frame - span.from) / fps) * Math.PI * 1.35) + 1) / 2
  const spread = interpolate(pulse, [0, 1], [0, 10])
  const glow = interpolate(pulse, [0, 1], [0.18, 0.42]) * appear * hide

  return (
    <div
      style={{
        position: 'absolute',
        left: area.x * SHOT_WIDTH,
        top: area.y * SHOT_HEIGHT,
        width: area.width * SHOT_WIDTH,
        height: area.height * SHOT_HEIGHT,
        borderRadius: 22,
        border: `3px solid rgba(0, 170, 255, ${0.9 * appear * hide})`,
        boxShadow: `0 0 0 ${spread}px rgba(0, 170, 255, ${glow})`,
        opacity: appear * hide,
      }}
    />
  )
}

/** Палец касается экрана за столько до перехода — чтобы нажатие успели заметить. */
const TAP_BEFORE_SECONDS = 0.62
const TAP_SECONDS = 0.5

/**
 * Нажатие: круг под пальцем и расходящаяся от него волна. Без этого переход выглядит
 * так, будто экран сменился сам — непонятно, что человек что-то сделал.
 */
function Tap({ span }: { span: Span }) {
  const frame = useCurrentFrame()
  const { fps } = useVideoConfig()

  const tap = span.scene.tap
  if (!tap) return null

  const start = span.to - Math.round(TAP_BEFORE_SECONDS * fps)
  const progress = interpolate(frame, [start, start + TAP_SECONDS * fps], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  })
  if (progress <= 0 || progress >= 1) return null

  const left = tap.x * SHOT_WIDTH
  const top = tap.y * SHOT_HEIGHT

  // Волна расходится и гаснет, кружок пальца держится чуть дольше и уходит следом.
  const ripple = interpolate(progress, [0, 1], [26, 132], {
    easing: Easing.out(Easing.cubic),
  })
  const rippleOpacity = interpolate(progress, [0, 0.8], [0.32, 0], {
    extrapolateRight: 'clamp',
  })
  const dot = interpolate(progress, [0, 0.35, 1], [0, 1, 0.85])
  const dotOpacity = interpolate(progress, [0, 0.15, 0.7, 1], [0, 1, 1, 0])

  return (
    <>
      <div
        style={{
          position: 'absolute',
          left,
          top,
          width: ripple,
          height: ripple,
          marginLeft: -ripple / 2,
          marginTop: -ripple / 2,
          borderRadius: '50%',
          backgroundColor: '#00aaff',
          opacity: rippleOpacity,
        }}
      />
      <div
        style={{
          position: 'absolute',
          left,
          top,
          width: 52,
          height: 52,
          marginLeft: -26,
          marginTop: -26,
          borderRadius: '50%',
          border: '3px solid rgba(255, 255, 255, 0.9)',
          backgroundColor: 'rgba(10, 10, 10, 0.28)',
          transform: `scale(${dot})`,
          opacity: dotOpacity,
        }}
      />
    </>
  )
}

/**
 * Строка состояния телефона: время и иконки связи. Без неё корпус выглядит пустой рамкой
 * вокруг картинки — а с ней читается как настоящее устройство.
 */
function StatusBar() {
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        height: STATUS_BAR_HEIGHT,
        padding: '0 34px',
        fontSize: 20,
        fontWeight: 700,
        color: '#0a0a0a',
        backgroundColor: '#fff',
      }}
    >
      <span>9:41</span>

      <span style={{ display: 'flex', alignItems: 'flex-end', gap: 7 }}>
        {/* Сигнал: четыре растущих штриха */}
        <svg width={22} height={16} viewBox="0 0 22 16">
          {[0, 1, 2, 3].map((bar) => (
            <rect
              key={bar}
              x={bar * 5.6}
              y={16 - (6 + bar * 3.2)}
              width={4}
              height={6 + bar * 3.2}
              rx={1.2}
              fill="#0a0a0a"
            />
          ))}
        </svg>

        {/* Wi-Fi: три дуги и точка */}
        <svg width={20} height={16} viewBox="0 0 20 16">
          <path
            d="M2 6.2a12 12 0 0 1 16 0M5 9.4a7.6 7.6 0 0 1 10 0"
            stroke="#0a0a0a"
            strokeWidth={2}
            strokeLinecap="round"
            fill="none"
          />
          <circle cx="10" cy="13" r="1.8" fill="#0a0a0a" />
        </svg>

        {/* Батарея заряжена на три четверти */}
        <svg width={28} height={16} viewBox="0 0 28 16">
          <rect
            x="1"
            y="2"
            width="23"
            height="12"
            rx="3.6"
            stroke="#0a0a0a"
            strokeOpacity={0.4}
            strokeWidth={1.6}
            fill="none"
          />
          <rect x="3.2" y="4.2" width="16" height="7.6" rx="2" fill="#0a0a0a" />
          <path d="M25.6 6v4a2.4 2.4 0 0 0 0-4Z" fill="#0a0a0a" fillOpacity={0.4} />
        </svg>
      </span>
    </div>
  )
}

function SideButton({ top }: { top: number }) {
  return (
    <div
      style={{
        position: 'absolute',
        left: -4,
        top,
        width: 5,
        height: 92,
        borderRadius: 3,
        backgroundColor: '#1c1c1e',
      }}
    />
  )
}
