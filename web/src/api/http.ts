// 请求封装：统一 JSON 编解码与错误处理；会话失效（401）时交给注册的回调跳转登录页。
// Request wrapper: JSON encode/decode and error handling; a 401 is handed to the registered callback to go to login.

export class ApiError extends Error {
  readonly status: number;
  readonly data: Record<string, unknown>;

  constructor(status: number, message: string, data?: Record<string, unknown>) {
    super(message);
    this.status = status;
    this.data = data ?? {};
  }
}

let unauthorizedHandler: (() => void) | null = null;

export function onUnauthorized(fn: () => void): void {
  unauthorizedHandler = fn;
}

async function request<T>(method: string, url: string, body?: unknown): Promise<T> {
  const init: RequestInit = { method, credentials: 'same-origin', headers: {} };
  if (body instanceof FormData) {
    init.body = body;
  } else if (body !== undefined) {
    (init.headers as Record<string, string>)['Content-Type'] = 'application/json';
    init.body = JSON.stringify(body);
  }
  let res: Response;
  try {
    res = await fetch(url, init);
  } catch {
    throw new ApiError(0, '网络连接失败，请检查网络');
  }
  const text = await res.text();
  let data: unknown = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = null;
    }
  }
  if (!res.ok) {
    const obj = (data && typeof data === 'object' ? data : {}) as Record<string, unknown>;
    const msg = typeof obj.error === 'string' ? obj.error : `请求失败（${res.status}）`;
    // 登录、注册、会话探测接口的 401 由调用方自行处理
    // 401s from login, register and session probing are handled by the caller.
    if (res.status === 401 && !url.startsWith('/api/auth/')) unauthorizedHandler?.();
    throw new ApiError(res.status, msg, obj);
  }
  return data as T;
}

export const http = {
  get: <T>(url: string) => request<T>('GET', url),
  post: <T>(url: string, body: unknown = {}) => request<T>('POST', url, body),
  put: <T>(url: string, body: unknown) => request<T>('PUT', url, body),
  del: <T>(url: string) => request<T>('DELETE', url),
  upload: <T>(url: string, file: Blob, filename = 'upload') => {
    const fd = new FormData();
    fd.append('file', file, file instanceof File ? file.name : filename);
    return request<T>('POST', url, fd);
  },
};

export function qs(params: Record<string, string | number | boolean | null | undefined>): string {
  const p = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== '' && v !== false) p.set(k, String(v));
  }
  const s = p.toString();
  return s ? `?${s}` : '';
}
