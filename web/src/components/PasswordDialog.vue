<script setup lang="ts">
// 修改密码弹窗；成功后其它设备的登录会失效。
// Change-password dialog; other devices are signed out on success.
import { reactive, watch } from 'vue';
import FormDialog from './FormDialog.vue';
import { authApi } from '@/api';
import { toast } from '@/utils/feedback';

const show = defineModel<boolean>('show', { required: true });
const form = reactive({ old: '', next: '', confirm: '' });
const errors = reactive({ next: '', confirm: '' });

watch(show, (v) => {
  if (!v) return;
  Object.assign(form, { old: '', next: '', confirm: '' });
  Object.assign(errors, { next: '', confirm: '' });
});

async function submit(): Promise<boolean> {
  errors.next = form.next.length < 6 ? '密码至少 6 位' : '';
  errors.confirm = !errors.next && form.next !== form.confirm ? '两次输入不一致' : '';
  if (errors.next || errors.confirm) return false;
  await authApi.changePassword(form.old, form.next);
  toast('密码已修改，其它设备需重新登录');
  return true;
}
</script>

<template>
  <FormDialog v-model:show="show" title="修改密码" :submit="submit">
    <van-field v-model="form.old" class="cw-field" label-align="top" label="原密码" type="password" autocomplete="current-password" autofocus />
    <van-field v-model="form.next" class="cw-field" label-align="top" label="新密码（至少 6 位）" type="password" maxlength="72" autocomplete="new-password" :error-message="errors.next" />
    <van-field v-model="form.confirm" class="cw-field" label-align="top" label="确认新密码" type="password" maxlength="72" autocomplete="new-password" :error-message="errors.confirm" />
  </FormDialog>
</template>
