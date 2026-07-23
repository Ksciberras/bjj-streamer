export async function api(path: string, options: RequestInit = {}) {
  const csrf = document.cookie
    .split('; ')
    .find((item) => item.startsWith('bjj_csrf='))
    ?.split('=')[1]

  const response = await fetch(path, {
    ...options,
    credentials: 'same-origin',
    headers: {
      'Content-Type': 'application/json',
      ...(csrf ? { 'X-CSRF-Token': decodeURIComponent(csrf) } : {}),
      ...options.headers,
    },
  })

  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: 'Request failed' }))
    throw new Error(body.error ?? 'Request failed')
  }

  return response.status === 204 ? null : response.json()
}

export function errorMessage(reason: unknown, fallback: string) {
  return reason instanceof Error ? reason.message : fallback
}
