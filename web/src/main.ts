import { createApp } from 'vue';
import { createPinia } from 'pinia';
import 'vant/lib/index.css';
import './styles/app.css';
import App from './App.vue';
import { router } from './router';
import { onUnauthorized } from './api/http';
import { useUserStore } from './stores/user';

const app = createApp(App);
const pinia = createPinia();
app.use(pinia).use(router);

// 会话过期：清空用户并带上当前地址跳转登录页，登录后回到原页面
// Session expired: clear the user and go to login with the current path, returning there after login.
onUnauthorized(() => {
  useUserStore(pinia).clear();
  const cur = router.currentRoute.value;
  if (cur.name !== 'login') router.replace({ name: 'login', query: { redirect: cur.fullPath } });
});

app.mount('#app');
