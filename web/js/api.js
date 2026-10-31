const bootEl = document.getElementById('bootstrap');
export const BOOT = JSON.parse(bootEl.textContent);
export const TOKEN = BOOT.token;
export const VERSION = BOOT.version;

export async function api(path, { method = 'GET', body, raw = false } = {}) {
  const headers = { 'X-NavMini-Token': TOKEN };
  let payload;
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
    payload = JSON.stringify(body);
  }
  const res = await fetch(path, { method, headers, body: payload });
  if (raw) return res;
  let data = null;
  try {
    data = await res.json();
  } catch {
    data = { error: res.statusText || 'invalid response' };
  }
  if (!res.ok) throw new Error(data && data.error ? data.error : res.statusText);
  return data;
}

export const apiGet = (path) => api(path);
export const apiPost = (path, body) => api(path, { method: 'POST', body });

export function qs(params) {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== '') q.set(k, v);
  }
  return q.toString();
}
