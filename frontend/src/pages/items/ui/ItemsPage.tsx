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
        <h1 className="text-[19px] font-bold">Мои объявления</h1>
        {/* В колонке кнопка растягивается сама — на узких окнах это привычная нижняя кнопка. */}
        <Button onClick={() => navigate('/items/new')}>Разместить объявление</Button>
      </div>

      <div className="px-4 pb-4">
        <ItemsList />
      </div>
    </Screen>
  )
}
