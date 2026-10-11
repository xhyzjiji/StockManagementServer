// 统一的提示与确认：基于 Vant 的函数式 API。
// Unified toasts and confirmations built on Vant's functional APIs.
import { showConfirmDialog, showToast } from 'vant';
import { ApiError } from '@/api/http';

export function toast(message: string, duration = 2200): void {
  showToast({ message, duration, wordBreak: 'break-word' });
}

// showError 展示接口错误；401 已由全局跳转登录处理，不重复提示。
// showError displays an API error; a 401 is already handled by the global redirect to login.
export function showError(e: unknown): void {
  if (e instanceof ApiError && e.status === 401) return;
  toast(e instanceof Error && e.message ? e.message : '操作失败');
}

export async function confirm(message: string, opts: { title?: string; okText?: string; danger?: boolean } = {}): Promise<boolean> {
  try {
    await showConfirmDialog({
      title: opts.title,
      message,
      confirmButtonText: opts.okText ?? '删除',
      confirmButtonColor: opts.danger === false ? undefined : 'var(--cw-out)',
      messageAlign: 'left',
    });
    return true;
  } catch {
    return false;
  }
}
