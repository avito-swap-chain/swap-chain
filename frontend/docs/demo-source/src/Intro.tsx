import { AbsoluteFill, interpolate, useCurrentFrame, useVideoConfig } from 'remotion'
import { SURFACE } from './Demo'

const BRAND = '#00aaff'
const INK = '#0a0a0a'
const INK_2 = '#575759'

/** Три вещи по кругу — вся идея продукта одной картинкой, до первого экрана. */
const RING = [
  { name: 'Даша', gives: 'Велосипед' },
  { name: 'Марк', gives: 'Наушники' },
  { name: 'Лена', gives: 'Фотоаппарат' },
]

export function Intro() {
  const frame = useCurrentFrame()
  const { fps } = useVideoConfig()

  const fade = interpolate(frame, [0, fps * 0.5], [0, 1], {
    extrapolateRight: 'clamp',
  })

  return (
    <AbsoluteFill
      style={{
        ...SURFACE,
        alignItems: 'center',
        justifyContent: 'center',
        gap: 72,
        opacity: fade,
      }}
    >
      <div style={{ textAlign: 'center', padding: '0 80px' }}>
        <h1
          style={{
            margin: 0,
            fontSize: 84,
            lineHeight: 1.05,
            color: INK,
            letterSpacing: -2.5,
          }}
        >
          Авито Обмен
        </h1>
        <p
          style={{
            margin: '28px 0 0',
            fontSize: 34,
            lineHeight: 1.3,
            color: INK_2,
          }}
        >
          Цепочка обмена: каждый отдаёт ненужное
          <br />и получает нужное. Без денег.
        </p>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: 24 }}>
        {RING.map((person, index) => {
          // Узлы проявляются по очереди — глаз успевает пройти цепочку целиком.
          const appear = interpolate(
            frame,
            [fps * (0.6 + index * 0.35), fps * (1.1 + index * 0.35)],
            [0, 1],
            { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' },
          )

          return (
            <div key={person.name} style={{ display: 'flex', alignItems: 'center', gap: 24 }}>
              <div style={{ opacity: appear, textAlign: 'center', width: 200 }}>
                <div
                  style={{
                    width: 96,
                    height: 96,
                    margin: '0 auto 14px',
                    borderRadius: '50%',
                    backgroundColor: '#fff',
                    border: `3px solid ${BRAND}`,
                    display: 'grid',
                    placeItems: 'center',
                    fontSize: 40,
                    fontWeight: 700,
                    color: BRAND,
                  }}
                >
                  {person.name[0]}
                </div>
                <div style={{ fontSize: 27, fontWeight: 700, color: INK }}>{person.name}</div>
                <div style={{ fontSize: 24, color: INK_2 }}>{person.gives}</div>
              </div>

              {index < RING.length - 1 && (
                <span
                  style={{
                    opacity: appear,
                    fontSize: 44,
                    color: BRAND,
                    marginBottom: 40,
                  }}
                >
                  →
                </span>
              )}
            </div>
          )
        })}
      </div>

      <p
        style={{
          margin: 0,
          fontSize: 27,
          color: INK_2,
          opacity: interpolate(frame, [fps * 2, fps * 2.6], [0, 1], {
            extrapolateLeft: 'clamp',
            extrapolateRight: 'clamp',
          }),
        }}
      >
        …и круг замыкается на первом участнике
      </p>
    </AbsoluteFill>
  )
}
