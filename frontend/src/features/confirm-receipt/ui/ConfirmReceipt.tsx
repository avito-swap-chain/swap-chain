import { useMutation, useQueryClient } from '@tanstack/react-query'
import { chainKeys, confirmReceipt } from '@/entities/chain'
import { Button } from '@/shared/ui'

/** Подтверждение получения вещи на финале обмена. */
export function ConfirmReceipt({ chainId }: { chainId: string }) {
  const queryClient = useQueryClient()
  const { mutate, isPending } = useMutation({
    mutationFn: () => confirmReceipt(chainId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: chainKeys.all }),
  })

  return (
    <Button fullWidth disabled={isPending} onClick={() => mutate()}>
      Подтвердить получение
    </Button>
  )
}
