<script setup lang="ts">
// 表单弹窗外壳：回车提交；submit 返回 false 或抛错时保持打开（抛错会提示错误信息）。
// Form dialog shell: Enter submits; it stays open when submit returns false or throws (errors are shown).
import { ref } from 'vue';
import { showError } from '@/utils/feedback';

const props = withDefaults(defineProps<{
  title?: string;
  okText?: string;
  cancelText?: string;
  danger?: boolean;
  submit: () => Promise<boolean | void> | boolean | void;
}>(), { title: '', okText: '确定', cancelText: '取消', danger: false });

const show = defineModel<boolean>('show', { required: true });
const emit = defineEmits<{ cancel: []; closed: [] }>();
const loading = ref(false);

async function onOk() {
  if (loading.value) return;
  loading.value = true;
  try {
    if ((await props.submit()) !== false) show.value = false;
  } catch (e) {
    showError(e);
  } finally {
    loading.value = false;
  }
}

// Vant 输入框在 van-form 之外会拦截回车，这里手动支持回车提交（跳过输入法组字中的回车）
// Vant inputs swallow Enter outside van-form, so Enter is handled here (skipping Enter while an IME is composing).
function onEnter(e: KeyboardEvent) {
  if (e.isComposing || (e.target as HTMLElement).tagName !== 'INPUT') return;
  e.preventDefault();
  onOk();
}

function onCancel() {
  show.value = false;
  emit('cancel');
}
</script>

<template>
  <van-dialog v-model:show="show" :title="title" :show-confirm-button="false" @closed="emit('closed')">
    <form class="cw-dialog-body" @submit.prevent="onOk" @keydown.enter="onEnter">
      <slot />
    </form>
    <template #footer>
      <div class="cw-dialog-actions">
        <van-button size="small" round @click="onCancel">{{ cancelText }}</van-button>
        <van-button size="small" round :type="danger ? 'danger' : 'primary'" :loading="loading" @click="onOk">{{ okText }}</van-button>
      </div>
    </template>
  </van-dialog>
</template>
