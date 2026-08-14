import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { chainKeys, dissolveChainsWithItem } from '@/entities/chain'
import { itemKeys, withdrawItem } from '@/entities/item'
import { isBackendConnected } from '@/shared/config/backend'
import { ActionError } from '@/shared/ui'

/**
 * Снять вещь с обмена. Спрашиваем подтверждение прямо в строке, без диалога: у действия
 * есть последствие для других людей — предложения с этой вещью отменятся, — и узнать о нём
 * после нажатия поздно. Вернуть всё как было одной кнопкой нельзя: желание придётся указать
 * заново, потому что снятие его стирает.
 */
export function WithdrawItem({ itemId, onDone }: { itemId: string; onDone?: () => void }) {
  const [asking, setAsking] = useState(false)
  const queryClient = useQueryClient()

  const withdraw = useMutation({
    mutationFn: async () => {
      const item = await withdrawItem(itemId)
      // Предложения с вещью отменяет бэкенд. На моках их некому распустить, а оставлять
      // вариант с вещью, которой в подборе уже нет, нельзя — в демо это выглядит поломкой.
      if (!isBackendConnected) dissolveChainsWithItem(itemId)
      return item
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: itemKeys.my() })
      queryClient.invalidateQueries({ queryKey: chainKeys.all })
      // Экран правки после снятия теряет смысл: желания у вещи больше нет, и править
      // на нём нечего — уводим туда, откуда пришли.
      onDone?.()
    },
  })

  const linkClass =
    'shrink-0 cursor-pointer rounded-sm text-[12.5px] font-semibold outline-offset-2 focus-visible:outline-2 focus-visible:outline-brand'

  if (!asking) {
    return (
      <button
        type="button"
        onClick={() => setAsking(true)}
        className={`${linkClass} text-ink-3 hover:text-ink-2`}
      >
        Снять с обмена
      </button>
    )
  }

  return (
    <span className="flex flex-col items-end gap-1">
      <span className="flex flex-wrap items-center justify-end gap-x-3 gap-y-1">
        <span className="text-[12.5px] text-ink-3">Предложения с ней отменятся</span>
        <button
          type="button"
          disabled={withdraw.isPending}
          onClick={() => withdraw.mutate()}
          className={`${linkClass} text-stop hover:opacity-80`}
        >
          Снять
        </button>
        <button
          type="button"
          onClick={() => setAsking(false)}
          className={`${linkClass} text-brand hover:opacity-80`}
        >
          Отмена
        </button>
      </span>
      {/* Вещь в собравшейся цепочке снять нельзя — бэкенд отвечает 409, и раньше это
          выглядело так, будто кнопка просто не работает. */}
      <ActionError
        error={withdraw.error}
        conflict="Вещь уже участвует в собравшейся цепочке — снять её нельзя"
      />
    </span>
  )
}
