import { APP_CONFIG } from '../shared/config/constants'
import type { DataSource } from '../shared/config/env'
import { MockDataDockGateway } from '../mocks/mock-gateway'
import type { DataDockGateway } from './gateway'
import { HttpDataDockGateway } from './http-gateway'

export function createDataDockGateway(source: DataSource = APP_CONFIG.dataSource): DataDockGateway {
  return source === 'api' ? new HttpDataDockGateway() : new MockDataDockGateway()
}
