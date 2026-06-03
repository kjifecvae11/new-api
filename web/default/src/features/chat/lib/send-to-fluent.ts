import { resolveServerAddress } from '@/lib/server-address'

export function sendToFluent(apiKey: string, serverAddress?: string): boolean {
  if (typeof window === 'undefined') {
    return false
  }

  const container = document.getElementById('fluent-new-api-container')
  if (!container) {
    return false
  }

  const baseUrl = serverAddress
    ? resolveServerAddress({ server_address: serverAddress })
    : resolveServerAddress()

  const payload = {
    id: 'new-api',
    baseUrl,
    apiKey: `sk-${apiKey}`,
  }

  container.dispatchEvent(
    new CustomEvent('fluent:prefill', {
      detail: payload,
    })
  )

  return true
}
