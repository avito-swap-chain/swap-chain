/**
 * Лок-ап «Avito Обмен»: знак Авито + название раздела.
 *
 * Геометрия знака — из официального SVG: четыре круга разного размера по двум диагоналям,
 * крупные зелёный и голубой против мелких фиолетового и красного. Не сетка 2×2.
 * Название раздела набрано тем же начертанием, что и «Avito», — как у вертикалей Авито
 * («Авито Доставка», «Авито Услуги»): это одно имя сервиса, а не бренд с подписью.
 */
export function BrandMark({ label = 'Обмен' }: { label?: string }) {
  return (
    /* Размеры сняты с шапки Авито: знак 30×30, надпись вровень с ним по оптическому весу. */
    <span className="flex items-center gap-2.5 text-[26px] leading-none font-bold">
      <svg viewBox="0 0 410 380" className="h-[30px] w-auto shrink-0" aria-hidden="true">
        <circle cx="122.965" cy="256.711" r="122.559" className="fill-accent-green" />
        <circle cx="306.803" cy="100.051" r="99.645" className="fill-brand" />
        <circle cx="335.574" cy="289.745" r="74.058" className="fill-accent-red" />
        <circle cx="146.404" cy="72.347" r="45.829" className="fill-accent-violet" />
      </svg>

      <span className="tracking-tight">Avito {label}</span>
    </span>
  )
}
