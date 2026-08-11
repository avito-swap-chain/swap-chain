import { useMutation, useQueryClient } from '@tanstack/react-query'
import { chainKeys, confirmReceipt } from '@/entities/chain'
import { ActionError, Button } from '@/shared/ui'

/** Подтверждение получения вещи на финале обмена. */
export function ConfirmReceipt({ chainId }: { chainId: string }) {
  const queryClient = useQueryClient()
  const { mutate, isPending, error } = useMutation({
    mutationFn: () => confirmReceipt(chainId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: chainKeys.all }),
  })

  return (
    <div className="flex flex-col gap-2">
      <Button fullWidth disabled={isPending} onClick={() => mutate()}>
        Подтвердить получение
      </Button>
      {/* 409 здесь штатный: бэкенд не принимает отметку, пока сотрудник ПВЗ не отправил
          вещь получателю. Без объяснения человек жмёт кнопку второй раз и третий. */}
      <ActionError
        error={error}
        conflict="Вещь ещё не выехала из пункта выдачи — отметить получение можно, когда она будет в пути"
      />
    </div>
  )
}
