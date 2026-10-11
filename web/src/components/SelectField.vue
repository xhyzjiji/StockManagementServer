<script setup lang="ts" generic="T extends string | number">
// 下拉选择字段：点击后从底部弹出 Picker。
// Select field: tapping opens a bottom Picker.
import { computed, ref } from 'vue';
import type { SelectOption } from './types';

const props = withDefaults(defineProps<{
  label?: string;
  options: SelectOption<T>[];
  placeholder?: string;
  plain?: boolean;
}>(), { label: '', placeholder: '请选择', plain: false });

const model = defineModel<T>({ required: true });
const show = ref(false);
const text = computed(() => props.options.find((o) => o.value === model.value)?.text ?? '');
const columns = computed(() => props.options.map((o) => ({ text: o.text, value: o.value })));
const selected = computed(() => [model.value]);

function onConfirm({ selectedValues }: { selectedValues: (string | number)[] }) {
  model.value = selectedValues[0] as T;
  show.value = false;
}
</script>

<template>
  <van-field
    class="cw-field"
    :class="{ plain }"
    :label="label || undefined"
    label-align="top"
    :model-value="text"
    :placeholder="placeholder"
    readonly
    right-icon="arrow-down"
    style="cursor: pointer"
    @click="show = true"
  />
  <van-popup v-model:show="show" position="bottom" round teleport="body">
    <van-picker :title="label || placeholder" :columns="columns" :model-value="selected" @confirm="onConfirm" @cancel="show = false" />
  </van-popup>
</template>
