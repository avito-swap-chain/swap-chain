import { Link, useNavigate } from 'react-router-dom'
import { Button, IconSparkle, Screen } from '@/shared/ui'
import { ItemsList } from '@/widgets/items-list'

/**
 * Раздел Авито «Мои объявления» — не наш сервис, а место, откуда в него заходят:
 * у объявления включают обмен. Вкладок здесь нет: верхний уровень навигации один,
 * и его держит меню кабинета в оболочке.
 */
export function ItemsPage() {
  const navigate = useNavigate()

  return (
    <Screen width="wide">
      <div className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between">
        {/* Заголовок раздела у Авито крупный — 32px на десктопе. */}
        <h1 className="text-[22px] leading-7 font-bold lg:text-[32px] lg:leading-10">
          Мои объявления
        </h1>
        {/* В колонке кнопка растягивается сама — на узких окнах это привычная нижняя кнопка. */}
        <Button onClick={() => navigate('/items/new')}>Разместить объявление</Button>
      </div>

      <div className="flex flex-col gap-3 px-4 pb-4">
        {/* Плашка над списком — как «Разберём гардероб с вами» у Авито: раздел объявлений
            это место, где рассказывают про новое в заполнении. У нас это разбор фотографии,
            и ссылка ведёт ровно туда, где он и работает. */}
        <Link
          to="/items/new"
          className="flex items-center gap-3 rounded-card bg-ok-bg px-4 py-3.5 outline-offset-2 transition-opacity hover:opacity-90 focus-visible:outline-2 focus-visible:outline-brand"
        >
          <IconSparkle size={22} className="shrink-0 text-ok" />
          <span className="text-[13.5px] leading-5">
            <b className="font-bold">Добавьте вещь по фото.</b> Нейросеть разберёт снимок и
            предложит описание, категорию и состояние&nbsp;→
          </span>
        </Link>

        <ItemsList />
      </div>
    </Screen>
  )
}
