export const CHANNEL_TYPE_CODEX = 57;
export const CHANNEL_TYPE_ANTHROPIC = 14;

const LOCAL_PROXY_HOSTS = new Set([
  'host.docker.internal',
  'localhost',
  '127.0.0.1',
  '::1',
]);

const includesClaudeCode = (value) =>
  String(value ?? '').toLowerCase().includes('claude-code');

const getUrlParts = (value) => {
  const text = String(value ?? '').trim();
  if (!text) return { hostname: '', port: '' };
  try {
    const parsed = new URL(text);
    return {
      hostname: parsed.hostname.toLowerCase(),
      port: parsed.port,
    };
  } catch (error) {
    return { hostname: '', port: '' };
  }
};

export const isClaudeCodeProxyChannel = (record) => {
  if (!record || record.children !== undefined) return false;
  if (record.type !== CHANNEL_TYPE_ANTHROPIC) return false;

  const baseUrl = String(record.base_url ?? '').trim();
  if (!baseUrl) return false;

  if (
    includesClaudeCode(record.name) ||
    includesClaudeCode(record.models) ||
    includesClaudeCode(baseUrl)
  ) {
    return true;
  }

  const { hostname, port } = getUrlParts(baseUrl);
  return LOCAL_PROXY_HOSTS.has(hostname) && port === '13140';
};

export const getChannelAccountInfoKind = (record) => {
  if (!record || record.children !== undefined) return null;
  if (record.type === CHANNEL_TYPE_CODEX) return 'codex';
  if (isClaudeCodeProxyChannel(record)) return 'claude-code';
  return null;
};

export const supportsChannelAccountInfo = (record) =>
  getChannelAccountInfoKind(record) !== null;
