import { Link } from 'react-router-dom'
import { SwitchPersona } from '@/features/switch-persona'
import { BrandMark, Button } from '@/shared/ui'

/**
 * Шапка Авито: бренд-лок-ап и поиск. Поиск — декорация, в демо ищет сервис подбора,
 * а не пользователь; поле выключено, чтобы это было видно без пояснений.
 *
 * Подложка — во всю ширину окна, содержимое — в том же контейнере, что и страница.
 * Шапка всегда в одну строку: на экране высотой 640 вторая строка съедала бы больше
 * сорока пикселей контента. На узких окнах декорации уходят целиком — поиск в демо
 * не работает, и отдавать ему половину единственной строки незачем; остаются лок-ап
 * и переключатель персон, без которого не показать цепочку с разных сторон.
 */
export function ShellHeader() {
  return (
    <header className="border-b border-line-2 bg-card">
      <div className="mx-auto flex w-full max-w-page items-center gap-2 px-3 py-2 sm:gap-2.5 sm:px-4 sm:py-2.5 lg:gap-5 lg:px-6 lg:py-3.5">
        <Link
          to="/"
          className="shrink-0 rounded-sm outline-offset-4 focus-visible:outline-2 focus-visible:outline-brand"
        >
          <BrandMark />
        </Link>

        <div className="flex min-w-0 flex-1 gap-2 max-lg:hidden lg:max-w-2xl">
          <input
            disabled
            aria-label="Поиск по объявлениям"
            placeholder="Поиск по объявлениям"
            className="min-w-0 flex-1 rounded-input border border-line bg-page px-3 py-1.5 text-[13px] text-ink-3 placeholder:text-ink-3 disabled:cursor-not-allowed sm:py-2 sm:text-[14px]"
          />
          <Button variant="dark" disabled className="px-3.5 py-2 text-[14px] max-sm:hidden">
            Найти
          </Button>
        </div>

        {/* Место аккаунта: в демо здесь переключатель персон — он же показывает, кто вошёл. */}
        <div className="ml-auto shrink-0">
          <SwitchPersona />
        </div>
      </div>
    </header>
  )
}
