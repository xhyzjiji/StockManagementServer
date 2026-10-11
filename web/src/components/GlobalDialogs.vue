<script setup lang="ts">
// 渲染 pickOption / promptText 两类全局弹窗。
// Renders the global pickOption / promptText dialogs.
import { computed, watch } from 'vue';
import FormDialog from './FormDialog.vue';
import { optionSheet, prompt } from '@/utils/dialogs';

const actions = computed(() => optionSheet.options.map((name) => ({ name })));

function settleOption(index: number) {
  const r = optionSheet.resolve;
  optionSheet.resolve = null;
  r?.(index);
}

function onSelect(_: unknown, index: number) {
  settleOption(index);
  optionSheet.show = false;
}

watch(() => optionSheet.show, (v) => { if (!v) settleOption(-1); });

function settlePrompt(v: string | null) {
  const r = prompt.resolve;
  prompt.resolve = null;
  r?.(v);
}

async function submitPrompt(): Promise<boolean> {
  const v = prompt.value.trim();
  prompt.error = '';
  const err = !v ? '请输入内容' : prompt.validate?.(v) ?? null;
  if (err) {
    prompt.error = err;
    return false;
  }
  await prompt.onOk?.(v);
  settlePrompt(v);
  return true;
}

watch(() => prompt.show, (v) => { if (!v) settlePrompt(null); });
</script>

<template>
  <van-action-sheet
    v-model:show="optionSheet.show"
    :title="optionSheet.title || undefined"
    :description="optionSheet.description || undefined"
    :actions="actions"
    :cancel-text="optionSheet.cancelText"
    @select="onSelect"
  />
  <FormDialog v-model:show="prompt.show" :title="prompt.title" :submit="submitPrompt">
    <van-field
      v-model="prompt.value"
      class="cw-field"
      :label="prompt.label"
      label-align="top"
      :placeholder="prompt.placeholder"
      :maxlength="prompt.maxlength"
      :inputmode="prompt.inputmode"
      :error-message="prompt.error"
      autofocus
    />
  </FormDialog>
</template>
