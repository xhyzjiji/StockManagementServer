<script setup lang="ts">
// 新建 / 编辑属性定义：名称、类型（文本 / 数字 / 单选 / 标签 / 图片）与候选值。
// Create / edit an attribute definition: name, type (text / number / choice / tags / image) and options.
import { computed, reactive, watch } from 'vue';
import FormDialog from './FormDialog.vue';
import type { AttrDraftRow } from './types';
import type { AttrType } from '@/api/types';
import { confirm } from '@/utils/feedback';
import { parseOptions } from '@/utils/format';

const props = defineProps<{ title: string; draft: AttrDraftRow; existingNames: string[] }>();
const show = defineModel<boolean>('show', { required: true });
const emit = defineEmits<{ ok: [draft: AttrDraftRow] }>();

const TYPES: [AttrType, string][] = [['TEXT', '文本'], ['NUMBER', '数字'], ['CHOICE', '单选'], ['TAGS', '标签'], ['IMAGE', '图片']];
const typeName = (t: AttrType) => TYPES.find(([k]) => k === t)?.[1] ?? t;

const form = reactive({ name: '', type: 'TEXT' as AttrType, options: '' });
const errors = reactive({ name: '', options: '' });
const hasOptions = computed(() => form.type === 'CHOICE' || form.type === 'TAGS');

watch(show, (v) => {
  if (!v) return;
  Object.assign(form, { name: props.draft.name, type: props.draft.type, options: props.draft.options.join(',') });
  Object.assign(errors, { name: '', options: '' });
});

async function submit(): Promise<boolean> {
  Object.assign(errors, { name: '', options: '' });
  const name = form.name.trim();
  if (!name) errors.name = '请输入属性名';
  else if (props.existingNames.includes(name)) errors.name = '属性名重复';
  const options = parseOptions(form.options);
  if (form.type === 'CHOICE' && options.length < 2) errors.options = '单选类型至少需要两个选项';
  if (form.type === 'TAGS' && options.length < 1) errors.options = '标签至少需要一个候选值';
  if (errors.name || errors.options) return false;
  const { id, originalType } = props.draft;
  if (id && originalType && originalType !== form.type) {
    const ok = await confirm(
      `属性 "${name}" 的类型将从「${typeName(originalType)}」改为「${typeName(form.type)}」，保存后所有物品已填写的内容（含图片）将被清除，确定切换？`,
      { okText: '切换' },
    );
    if (!ok) return false;
  }
  emit('ok', { ...props.draft, name, type: form.type, options: hasOptions.value ? options : [] });
  return true;
}
</script>

<template>
  <FormDialog v-model:show="show" :title="title" :submit="submit">
    <van-field v-model="form.name" class="cw-field" label-align="top" label="属性名" maxlength="64" placeholder="属性名（如：口味、单价、参考图）" :error-message="errors.name" autofocus />
    <div class="van-field__label muted small" style="margin-bottom: 6px">类型</div>
    <div class="chips wrap">
      <div v-for="[k, label] in TYPES" :key="k" class="chip" :class="{ active: form.type === k }" @click="form.type = k">{{ label }}</div>
    </div>
    <van-field
      v-if="hasOptions" v-model="form.options" class="cw-field" label-align="top"
      :label="form.type === 'TAGS' ? '候选标签，用逗号分隔（如：棉布,蓝色,条纹）' : '选项，用逗号分隔（如：有糖,无糖）'"
      :error-message="errors.options"
    />
  </FormDialog>
</template>
