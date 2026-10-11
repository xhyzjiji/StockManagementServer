<script setup lang="ts">
// 登录 / 注册（注册入口由后端 allow_register 控制）。
// Login / registration (registration is shown only when the backend allows it).
import { onMounted, reactive, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { authApi } from '@/api';
import { ApiError } from '@/api/http';
import { safeRedirect } from '@/router';
import { useUserStore } from '@/stores/user';

const route = useRoute();
const router = useRouter();
const store = useUserStore();

const allowRegister = ref(false);
const registering = ref(false);
const submitting = ref(false);
const error = ref('');
const form = reactive({ username: '', password: '' });

onMounted(async () => {
  try {
    allowRegister.value = (await authApi.config()).allow_register;
  } catch {
    // 拿不到配置时只提供登录
    // Without the config only login is offered.
  }
});

function toggleMode() {
  registering.value = !registering.value;
  error.value = '';
}

// Vant 输入框在 van-form 之外会拦截回车，这里手动支持回车登录
// Vant inputs swallow Enter outside van-form, so Enter-to-login is handled here.
function onEnter(e: KeyboardEvent) {
  if (e.isComposing || (e.target as HTMLElement).tagName !== 'INPUT') return;
  e.preventDefault();
  submit();
}

async function submit() {
  if (submitting.value) return;
  error.value = '';
  const username = form.username.trim();
  if (!username || !form.password) {
    error.value = '请输入用户名和密码';
    return;
  }
  submitting.value = true;
  try {
    const res = registering.value
      ? await authApi.register(username, form.password)
      : await authApi.login(username, form.password);
    // 校验浏览器是否真的保存了登录 cookie（HTTP 下开启 cookie_secure 时会被丢弃）
    // Verify the browser actually kept the login cookie (it is dropped over HTTP when cookie_secure is on).
    try {
      await authApi.me();
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        error.value = location.protocol === 'http:'
          ? '登录成功，但浏览器没有保存登录凭证：通过 http 访问时请把后端配置 auth.cookie_secure 设为 false'
          : '登录成功，但浏览器没有保存登录凭证，请检查浏览器是否禁用了 Cookie';
        return;
      }
      throw e;
    }
    store.set(res.user);
    await router.replace(safeRedirect(route.query.redirect));
  } catch (e) {
    error.value = e instanceof Error ? e.message : '登录失败';
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <div class="login-wrap">
    <div class="card login-card">
      <div class="brand"><van-icon name="gift-o" /><h1>造物仓</h1></div>
      <p class="muted" style="margin: 0 0 24px">管好材料库，记录每一次创作</p>
      <form @submit.prevent="submit" @keydown.enter="onEnter">
        <van-field v-model="form.username" class="cw-field" label-align="top" label="用户名" maxlength="64" autocomplete="username" autofocus />
        <van-field
          v-model="form.password" class="cw-field" label-align="top" label="密码" type="password" maxlength="72"
          :autocomplete="registering ? 'new-password' : 'current-password'"
        />
        <div class="out small" style="min-height: 20px; margin-bottom: 8px">{{ error }}</div>
        <van-button type="primary" round block native-type="submit" :loading="submitting">{{ registering ? '注册并登录' : '登录' }}</van-button>
        <van-button v-if="allowRegister" style="margin-top: 8px" round block plain hairline type="primary" native-type="button" @click="toggleMode">
          {{ registering ? '已有账号？登录' : '没有账号？注册' }}
        </van-button>
      </form>
    </div>
  </div>
</template>
