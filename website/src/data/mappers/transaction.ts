import type { ApiTransaction } from '../contracts/transaction'
import type { TransactionState } from '../../entities/query'

export function mapTransaction(value: ApiTransaction): TransactionState {
  return {
    id: value.id,
    connectionId: value.connectionId,
    state: value.state === 'rolled_back' ? 'rolled-back' : value.state,
    startedAt: value.startedAt,
    lastActivityAt: value.lastActivityAt,
    expiresAt: value.expiresAt,
    completedAt: value.completedAt,
    savepoints: value.savepoints.map((savepoint) => savepoint.name),
  }
}
