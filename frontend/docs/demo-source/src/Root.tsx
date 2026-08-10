import { Composition } from 'remotion'
import { Demo } from './Demo'
import { totalFrames } from './scenes'

const FPS = 30

/**
 * Вертикальный кадр 1080×1920: ролик смотрят с телефона в телеграме,
 * а экраны продукта у нас тоже вертикальные — горизонтальный кадр оставил бы поля по бокам.
 */
export function Root() {
  return (
    <Composition
      id="Demo"
      component={Demo}
      durationInFrames={totalFrames(FPS)}
      fps={FPS}
      width={1080}
      height={1920}
    />
  )
}
