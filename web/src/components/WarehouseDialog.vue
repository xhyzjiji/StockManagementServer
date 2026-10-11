<script setup lang="ts">
// 新建 / 编辑仓库弹窗。
// Create / edit warehouse dialog.
import { reactive, watch } from 'vue';
import FormDialog from './FormDialog.vue';

const props = withDefaults(defineProps<{
  title: string;
  name?: string;
  remark?: string;
  onOk: (name: string, remark: string) => Promise<void>;
}>(), { name: '', remark: '' });
const show = defineModel<boolean>('show', { required: true });

const form = reactive({ name: '', remark: '', error: '' });

watch(show, (v) => {
  if (v) Object.assign(form, { name: props.name, remark: props.remark, error: '' });
});

async function submit(): Promise<boolean> {
  const name = form.name.trim();
  form.error = name ? '' : '请输入仓库名称';
  if (!name) return false;
  await props.onOk(name, form.remark.trim());
  return true;
}
</script>

<template>
  <FormDialog v-model:show="show" :title="title" :submit="submit">
    <van-field v-model="form.name" class="cw-field" label-align="top" label="仓库名称" maxlength="128" placeholder="仓库名称" :error-message="form.error" autofocus />
    <van-field v-model="form.remark" class="cw-field" label-align="top" label="备注" type="textarea" rows="3" autosize maxlength="1000" placeholder="备注（可选）" />
  </FormDialog>
</template>
