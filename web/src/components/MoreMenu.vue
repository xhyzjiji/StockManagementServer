<script setup lang="ts">
// 溢出菜单：点击触发器弹出操作列表。
// Overflow menu: tapping the trigger opens a list of actions.
import { computed, ref } from 'vue';
import type { MenuItem } from './types';

const props = withDefaults(defineProps<{ items: MenuItem[]; icon?: string }>(), { icon: 'ellipsis' });
const show = ref(false);
const actions = computed(() => props.items.map((it) => ({ text: it.text, color: it.danger ? 'var(--cw-out)' : undefined })));

function onSelect(_: unknown, index: number) {
  props.items[index]?.onClick();
}
</script>

<template>
  <van-popover v-model:show="show" :actions="actions" placement="bottom-end" @select="onSelect">
    <template #reference>
      <slot><van-icon :name="icon" /></slot>
    </template>
  </van-popover>
</template>
