export function createApi(base = '') {
  async function request(url, options = {}, timeout = 8000) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeout);
    try { return await fetch(`${base}${url}`, { ...options, signal: controller.signal }); }
    finally { clearTimeout(timer); }
  }
  return { request };
}
