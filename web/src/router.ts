// 路由：history 模式（部署时需把未知路径回退到 index.html）；除登录页外都需要登录。
// Routes in history mode (deployments must fall back to index.html); everything except login requires a session.
import { createRouter, createWebHistory } from 'vue-router';
import { useUserStore } from '@/stores/user';

declare module 'vue-router' {
  interface RouteMeta {
    public?: boolean;
  }
}

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { public: true } },
    { path: '/', name: 'home', component: () => import('@/views/HomeView.vue') },
    { path: '/warehouses', name: 'warehouses', component: () => import('@/views/WarehousesView.vue') },
    { path: '/works', name: 'works', component: () => import('@/views/WorksView.vue') },
    { path: '/works/new', name: 'work-new', component: () => import('@/views/WorkCreateView.vue') },
    { path: '/works/:id(\\d+)/edit', name: 'work-edit', component: () => import('@/views/WorkCreateView.vue') },
    { path: '/works/:id(\\d+)', name: 'work', component: () => import('@/views/WorkDetailView.vue') },
    { path: '/inspiration', name: 'inspiration', component: () => import('@/views/InspirationView.vue') },
    { path: '/bin', name: 'warehouse-bin', component: () => import('@/views/WarehouseBinView.vue') },
    { path: '/w/:id(\\d+)', name: 'overview', component: () => import('@/views/OverviewView.vue') },
    { path: '/w/:id(\\d+)/attrs', name: 'attrs', component: () => import('@/views/AttrsView.vue') },
    { path: '/w/:id(\\d+)/records', name: 'warehouse-records', component: () => import('@/views/RecordsView.vue'), props: { scope: 'warehouse' } },
    { path: '/w/:id(\\d+)/stats', name: 'stats', component: () => import('@/views/StatsView.vue') },
    { path: '/w/:id(\\d+)/bin', name: 'item-bin', component: () => import('@/views/ItemBinView.vue') },
    { path: '/item/new', name: 'item-new', component: () => import('@/views/ItemEditView.vue') },
    { path: '/item/:id(\\d+)', name: 'item', component: () => import('@/views/ItemDetailView.vue') },
    { path: '/item/:id(\\d+)/edit', name: 'item-edit', component: () => import('@/views/ItemEditView.vue') },
    { path: '/item/:id(\\d+)/records', name: 'item-records', component: () => import('@/views/RecordsView.vue'), props: { scope: 'item' } },
    { path: '/:pathMatch(.*)*', name: 'not-found', component: () => import('@/views/NotFoundView.vue') },
  ],
  scrollBehavior: () => ({ top: 0 }),
});

// safeRedirect 只接受站内路径，防止登录后被带到外部地址。
// safeRedirect only accepts in-app paths so login cannot redirect to an external site.
export function safeRedirect(raw: unknown): string {
  return typeof raw === 'string' && raw.startsWith('/') && !raw.startsWith('//') && !raw.startsWith('/login') ? raw : '/';
}

router.beforeEach(async (to) => {
  const store = useUserStore();
  let user;
  try {
    user = await store.ensure();
  } catch {
    // 网络异常时放行，由页面自身的请求报错
    // On network errors let the page load and report its own request failures.
    return true;
  }
  if (to.name === 'login') return user ? safeRedirect(to.query.redirect) : true;
  if (!to.meta.public && !user) return { name: 'login', query: { redirect: to.fullPath } };
  return true;
});
