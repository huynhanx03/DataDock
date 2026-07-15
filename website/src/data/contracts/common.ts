export type ApiEnvelope<T> = {
  data: T
}

export type ApiErrorEnvelope = {
  error?: {
    code?: string
    message?: string
    requestId?: string
  }
}
