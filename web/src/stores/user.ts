// 当前登录用户：首次进入受保护页面时向后端探测一次会话。
// Current user: the session is probed once when the first protected page is entered.
import { defineStore } from 'pinia';
import { ref } from 'vue';
import { authApi } from '@/api';
import { ApiError } from '@/api/http';
import type { User } from '@/api/types';

export const useUserStore = defineStore('user', () => {
  const user = ref<User | null>(null);
  const checked = ref(false);

  // ensure 返回当前用户；未登录返回 null，网络等其它错误向上抛出。
  // ensure resolves to the current user, or null when logged out; other errors (e.g. network) are thrown.
  async function ensure(): Promise<User | null> {
    if (checked.value) return user.value;
    try {
      user.value = (await authApi.me()).user;
    } catch (e) {
      if (!(e instanceof ApiError && e.status === 401)) throw e;
      user.value = null;
    }
    checked.value = true;
    return user.value;
  }

  function set(u: User): void {
    user.value = u;
    checked.value = true;
  }

  function clear(): void {
    user.value = null;
    checked.value = true;
  }

  return { user, ensure, set, clear };
});
