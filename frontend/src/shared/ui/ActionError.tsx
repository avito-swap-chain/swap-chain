import { describeError } from '../api/describeError'

/**
 * Почему действие не получилось — строкой рядом с кнопкой, которую нажали. Не всплывающее
 * окно и не общий тост: человек смотрит туда, куда только что нажал, и там же должен
 * увидеть ответ.
 *
 * `role="alert"` — чтобы экранный читатель озвучил появившийся текст: кнопка выходит
 * из «нажата», и без озвучки результат остаётся незамеченным.
 */
export function ActionError({ error, conflict }: { error: unknown; conflict?: string }) {
  if (!error) return null

  return (
    <p role="alert" className="text-[12.5px] leading-4 font-semibold text-stop">
      {describeError(error, conflict)}
    </p>
  )
}
