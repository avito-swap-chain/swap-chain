import { useMutation, useQueryClient } from '@tanstack/react-query'
import { chainKeys, leaveChain } from '@/entities/chain'
import { Button } from '@/shared/ui'

/** Выход из цепочки до общего подтверждения — без штрафа (гипотеза H5). */
export function LeaveChain({ chainId }: { chainId: string }) {
  const queryClient = useQueryClient()
  const { mutate, isPending } = useMutation({
    mutationFn: () => leaveChain(chainId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: chainKeys.all }),
  })

  return (
    <Button variant="danger" fullWidth disabled={isPending} onClick={() => mutate()}>
      Выйти из цепочки
    </Button>
  )
}
