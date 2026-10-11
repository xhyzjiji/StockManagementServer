<script setup lang="ts">
import { computed } from 'vue';
import { tagColor } from '@/utils/format';

const props = defineProps<{ tag: string }>();
const color = computed(() => tagColor(props.tag));
</script>

<template>
  <van-tag round :color="color.bg" :text-color="color.fg">{{ tag }}</van-tag>
</template>
