<script setup lang="ts">
// 饼图：slices 为 [{label, value}]，按 PIE_COLORS 依次着色。
// Pie chart: slices are [{label, value}], colored in PIE_COLORS order.
import { computed } from 'vue';
import { PIE_COLORS } from './palette';

const props = withDefaults(defineProps<{ slices: { label: string; value: number }[]; size?: number }>(), { size: 180 });

const paths = computed(() => {
  const r = props.size / 2;
  const total = props.slices.reduce((a, b) => a + b.value, 0);
  if (total <= 0 || props.slices.length <= 1) return [];
  let angle = -Math.PI / 2;
  return props.slices.map((sl, i) => {
    const a = (sl.value / total) * Math.PI * 2;
    const x1 = r + r * Math.cos(angle);
    const y1 = r + r * Math.sin(angle);
    angle += a;
    const x2 = r + r * Math.cos(angle);
    const y2 = r + r * Math.sin(angle);
    return {
      d: `M${r},${r} L${x1},${y1} A${r},${r} 0 ${a > Math.PI ? 1 : 0} 1 ${x2},${y2} Z`,
      fill: PIE_COLORS[i % PIE_COLORS.length],
      label: sl.label,
    };
  });
});
</script>

<template>
  <svg :width="size" :height="size" :viewBox="`0 0 ${size} ${size}`">
    <circle v-if="slices.length === 1 && slices[0].value > 0" :cx="size / 2" :cy="size / 2" :r="size / 2" :fill="PIE_COLORS[0]">
      <title>{{ slices[0].label }}</title>
    </circle>
    <path v-for="(p, i) in paths" :key="i" :d="p.d" :fill="p.fill" stroke="#fff" stroke-width="1">
      <title>{{ p.label }}</title>
    </path>
  </svg>
</template>
