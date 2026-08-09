import { useNavigate } from 'react-router-dom'
import { Button, Screen } from '@/shared/ui'
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
        {/* Главное действие в системе Авито чёрное: азур у них означает ссылку.
            В колонке кнопка растягивается сама — на узких окнах это привычная нижняя кнопка. */}
        <Button variant="dark" onClick={() => navigate('/items/new')}>
          Разместить объявление
        </Button>
      </div>

      <div className="px-4 pb-4">
        <ItemsList />
      </div>
    </Screen>
  )
}
