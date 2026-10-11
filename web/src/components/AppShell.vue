<script setup lang="ts">
// 门户外壳：桌面左侧栏 + 顶栏，手机顶栏 + 底栏；承载首页 / 仓库 / 作品 / 灵感。
// Portal shell: desktop left sidebar + top bar, mobile top bar + bottom tabs;
// hosts home / warehouses / works / inspiration.
import { computed, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import MoreMenu from '@/components/MoreMenu.vue';
import PasswordDialog from '@/components/PasswordDialog.vue';
import type { MenuItem } from '@/components/types';
import { authApi } from '@/api';
import { useUserStore } from '@/stores/user';
import { showError } from '@/utils/feedback';

const route = useRoute();
const router = useRouter();
const store = useUserStore();
const showPassword = ref(false);

const nav = [
  { name: 'home', path: '/', label: '首页', icon: 'home-o' },
  { name: 'warehouses', path: '/warehouses', label: '我的仓库', icon: 'shop-o' },
  { name: 'works', path: '/works', label: '我的作品', icon: 'photo-o' },
  { name: 'inspiration', path: '/inspiration', label: '创作灵感', icon: 'bulb-o' },
] as const;

const active = computed(() => {
  const n = String(route.name ?? '');
  if (n === 'home') return 'home';
  if (n === 'warehouses' || n === 'warehouse-bin') return 'warehouses';
  if (n === 'works' || n === 'work' || n === 'work-new' || n === 'work-edit') return 'works';
  if (n === 'inspiration') return 'inspiration';
  return '';
});

const pageTitle = computed(() => nav.find((i) => i.name === active.value)?.label ?? '造物仓');

const accountMenu = computed<MenuItem[]>(() => [
  { text: `当前账号：${store.user?.username ?? ''}`, onClick: () => {} },
  { text: '修改密码', onClick: () => { showPassword.value = true; } },
  { text: '退出登录', danger: true, onClick: logout },
]);

async function logout() {
  try {
    await authApi.logout();
  } catch (e) {
    showError(e);
  }
  store.clear();
  router.replace('/login');
}
</script>

<template>
  <div class="shell">
    <aside class="shell-side">
      <div class="shell-brand" @click="router.push('/')">
        <van-icon name="gift-o" />
        <div>
          <div class="shell-brand-name">造物仓</div>
          <div class="shell-brand-sub">创作材料库</div>
        </div>
      </div>
      <nav class="shell-nav">
        <router-link
          v-for="item in nav" :key="item.name" :to="item.path"
          class="shell-nav-item" :class="{ active: active === item.name }"
        >
          <van-icon :name="item.icon" />
          <span>{{ item.label }}</span>
        </router-link>
      </nav>
    </aside>

    <div class="shell-main">
      <header class="shell-top">
        <div class="shell-top-title">{{ pageTitle }}</div>
        <div class="shell-top-actions">
          <slot name="actions" />
          <MoreMenu :items="accountMenu" icon="user-o" />
        </div>
      </header>
      <div class="shell-body">
        <slot />
      </div>
      <slot name="footer" />
    </div>

    <nav class="shell-tabbar">
      <router-link
        v-for="item in nav" :key="item.name" :to="item.path"
        class="shell-tab" :class="{ active: active === item.name }"
      >
        <van-icon :name="item.icon" />
        <span>{{ item.label }}</span>
      </router-link>
    </nav>
  </div>
  <PasswordDialog v-model:show="showPassword" />
</template>
