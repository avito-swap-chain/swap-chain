import { useMutation, useQueryClient } from '@tanstack/react-query'
import { chainKeys, respondToChain, type ChainDecision } from '@/entities/chain'
import { Button } from '@/shared/ui'

/**
 * Ответ на один вариант обмена: «нравится» — согласие на него, а не заключённая сделка;
 * «не подходит» — отказ от этого варианта, остальные предложения человека остаются.
 * Кнопки стоят в строку: ответить надо быстро и в списке, и на экране цепочки.
 */
export function RespondToExchange({ chainId }: { chainId: string }) {
  const queryClient = useQueryClient()
  const { mutate, isPending } = useMutation({
    mutationFn: (decision: ChainDecision) => respondToChain(chainId, decision),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: chainKeys.all }),
  })

  return (
    <div className="flex gap-2.5">
      <Button className="flex-1" disabled={isPending} onClick={() => mutate('like')}>
        Нравится
      </Button>
      <Button
        variant="secondary"
        className="flex-1"
        disabled={isPending}
        onClick={() => mutate('dislike')}
      >
        Не подходит
      </Button>
    </div>
  )
}
