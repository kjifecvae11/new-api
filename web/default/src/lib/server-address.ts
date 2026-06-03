/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
type StatusAddressSource = {
  [key: string]: unknown
  server_address?: unknown
  serverAddress?: unknown
  data?: Record<string, unknown> | null
}

function cleanAddress(value: unknown): string {
  if (typeof value !== 'string') return ''
  const cleaned = value.trim().replace(/\/+$/, '')
  const lower = cleaned.toLowerCase()
  if (lower.startsWith('wss://')) {
    return `https://${cleaned.slice('wss://'.length)}`
  }
  if (lower.startsWith('ws://')) {
    return `http://${cleaned.slice('ws://'.length)}`
  }
  return cleaned
}

function getAddressOrigin(value: string): string {
  try {
    return new URL(value).origin
  } catch {
    return cleanAddress(value)
  }
}

function isSameOrigin(first: string, second: string): boolean {
  const left = getAddressOrigin(first).toLowerCase()
  const right = getAddressOrigin(second).toLowerCase()
  return left !== '' && left === right
}

function isDefaultLocalServerAddress(value: string): boolean {
  try {
    const url = new URL(value)
    const host = url.hostname.toLowerCase()
    return (
      url.protocol === 'http:' &&
      url.port === '3000' &&
      (host === 'localhost' ||
        host === '127.0.0.1' ||
        host === '[::1]' ||
        host === '::1')
    )
  } catch {
    return cleanAddress(value).toLowerCase() === 'http://localhost:3000'
  }
}

function getWindowOrigin(): string {
  if (typeof window === 'undefined') return ''
  return window.location.origin
}

export function extractConfiguredServerAddress(
  status: StatusAddressSource | null | undefined
): string {
  return cleanAddress(
    status?.server_address ??
      status?.serverAddress ??
      status?.data?.server_address ??
      status?.data?.serverAddress
  )
}

export function resolveServerAddress(
  status?: StatusAddressSource | null,
  fallbackOrigin = getWindowOrigin()
): string {
  const configured = extractConfiguredServerAddress(status)
  const fallback = cleanAddress(fallbackOrigin)

  if (!configured) return fallback

  if (
    isDefaultLocalServerAddress(configured) &&
    fallback &&
    !isSameOrigin(configured, fallback)
  ) {
    return import.meta.env.DEV ? configured : fallback
  }

  return configured
}
