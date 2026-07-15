import { APP_CONFIG } from '../shared/config/constants'
import { buildApiUrl } from '../shared/config/api-endpoints'
import type { ApiEnvelope, ApiErrorEnvelope } from './contracts/common'
import { GatewayError } from './gateway'

type RequestOptions = RequestInit & {
  signal?: AbortSignal
}

export class ApiClient {
  async request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const controller = new AbortController()
    let timedOut = false
    const abort = () => controller.abort(options.signal?.reason)
    options.signal?.addEventListener('abort', abort, { once: true })
    if (options.signal?.aborted) controller.abort(options.signal.reason)
    const timeout = window.setTimeout(() => {
      timedOut = true
      controller.abort('timeout')
    }, APP_CONFIG.api.requestTimeoutMs)
    const headers = new Headers(options.headers)
    headers.set('Accept', 'application/json')
    if (options.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')

    try {
      const response = await fetch(buildApiUrl(path), { ...options, headers, signal: controller.signal })
      const text = await response.text()
      let body: ApiEnvelope<T> | ApiErrorEnvelope | undefined
      if (text) {
        try {
          body = JSON.parse(text) as ApiEnvelope<T> | ApiErrorEnvelope
        } catch {
          throw new GatewayError('The API returned an invalid JSON response', 'invalid_response', response.status)
        }
      }
      if (!response.ok) {
        const error = body as ApiErrorEnvelope | undefined
        throw new GatewayError(
          error?.error?.message || `Request failed (${response.status})`,
          error?.error?.code || 'http_error',
          response.status,
          error?.error?.requestId,
        )
      }
      if (!body || !('data' in body)) return undefined as T
      return body.data
    } catch (error) {
      if (error instanceof GatewayError) throw error
      if (controller.signal.aborted) {
        throw new GatewayError(timedOut ? 'Request timed out' : 'Request was cancelled', timedOut ? 'request_timeout' : 'request_aborted')
      }
      throw new GatewayError(error instanceof Error ? error.message : 'Network request failed', 'network_error')
    } finally {
      window.clearTimeout(timeout)
      options.signal?.removeEventListener('abort', abort)
    }
  }
}
