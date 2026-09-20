export function appVersion(): string {
  const raw = import.meta.env.VITE_APP_VERSION
  if (typeof raw === 'string' && raw.trim() !== '') {
    return raw.trim()
  }
  return 'dev'
}
