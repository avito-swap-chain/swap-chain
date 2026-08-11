export type {
  Chain,
  ChainDecision,
  ChainStatus,
  ChainParticipant,
  ParticipantStatus,
} from './model/types'
export {
  chainKeys,
  getChain,
  getMyChains,
  respondToChain,
  leaveChain,
  confirmReceipt,
  dissolveChainsWithItem,
} from './api/chainApi'
// `displayName` и `isNeighbour` наружу не отдаём: подпись участника и выделение соседа —
// дело самих карточек цепочки, а они живут здесь же.
export {
  findMe,
  findNeighbours,
  findDecliner,
  needsMyAction,
  countConfirmed,
  countReceipts,
  type Neighbours,
} from './lib/participants'
export { isOpenOffer, countVariantsWithItem, cancelReason } from './lib/offers'
export { ChainCard } from './ui/ChainCard'
export { ExchangeSummary } from './ui/ExchangeSummary'
export { ChainRibbon } from './ui/ChainRibbon'
export { ParticipantStatusList } from './ui/ParticipantStatusList'
export { DealStatusLabel } from './ui/DealStatusLabel'
