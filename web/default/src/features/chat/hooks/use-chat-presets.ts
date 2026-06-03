import { useMemo } from 'react'
import { useStatus } from '@/hooks/use-status'
import { resolveServerAddress } from '@/lib/server-address'
import type { SystemStatus } from '@/features/auth/types'
import {
  type ChatPreset,
  parseChatConfig,
  type RawChatConfig,
} from '../lib/chat-links'

function getStoredStatusChats(): RawChatConfig {
  if (typeof window === 'undefined') return undefined
  try {
    const raw = window.localStorage.getItem('status')
    if (!raw) return undefined
    const parsed = JSON.parse(raw)
    return parsed?.chats ?? parsed?.Chats
  } catch {
    return undefined
  }
}

function extractChats(status: SystemStatus | null): RawChatConfig {
  const raw =
    status?.Chats ?? status?.chats ?? status?.data?.Chats ?? status?.data?.chats

  return (raw as RawChatConfig) ?? getStoredStatusChats()
}

export function useChatPresets(): {
  chatPresets: ChatPreset[]
  serverAddress: string
} {
  const { status } = useStatus()

  const serverAddress = useMemo(() => resolveServerAddress(status), [status])

  const chatPresets = useMemo(() => {
    const raw = extractChats(status)
    return parseChatConfig(raw)
  }, [status])

  return {
    chatPresets,
    serverAddress,
  }
}
