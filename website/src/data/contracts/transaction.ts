export type ApiTransaction = {
  id: string
  connectionId: string
  state: 'active' | 'committed' | 'rolled_back' | 'expired'
  startedAt: string
  lastActivityAt: string
  expiresAt: string
  completedAt?: string
  savepoints: Array<{
    name: string
    createdAt: string
  }>
}
