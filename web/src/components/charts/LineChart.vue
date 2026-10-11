<script setup lang="ts">
// 单价折线图：横轴按序号等距排列（与 App 一致），纵轴自动取范围。
// Price line chart: points are spaced evenly by index (as in the App) with an auto-scaled y axis.
import { computed } from 'vue';
import type { PricePoint } from '@/api/types';
import { fmtDayShort, fmtMoney } from '@/utils/format';

const props = withDefaults(defineProps<{ points: PricePoint[]; width?: number; height?: number }>(), { width: 640, height: 220 });

const PAD_L = 52;
const PAD_R = 16;
const PAD_T = 16;
const PAD_B = 30;

const chart = computed(() => {
  const { points, width, height } = props;
  const prices = points.map((p) => p.price);
  let min = Math.min(...prices);
  let max = Math.max(...prices);
  if (min === max) {
    min = Math.max(0, min - 1);
    max += 1;
  }
  const span = max - min;
  min = Math.max(0, min - span * 0.1);
  max += span * 0.1;
  const plotW = width - PAD_L - PAD_R;
  const plotH = height - PAD_T - PAD_B;
  const xOf = (i: number) => PAD_L + (points.length === 1 ? plotW / 2 : (i * plotW) / (points.length - 1));
  const yOf = (v: number) => PAD_T + plotH - ((v - min) / (max - min)) * plotH;
  const grid = [0, 1, 2, 3, 4].map((i) => {
    const v = min + ((max - min) * i) / 4;
    return { y: yOf(v), label: fmtMoney(v) };
  });
  const labelEvery = Math.max(1, Math.ceil(points.length / 6));
  const dots = points.map((p, i) => ({
    x: xOf(i), y: yOf(p.price),
    title: `${fmtDayShort(p.time)}  ¥${fmtMoney(p.price)}`,
    label: i % labelEvery === 0 || i === points.length - 1 ? fmtDayShort(p.time) : '',
  }));
  const d = dots.map((p, i) => `${i ? 'L' : 'M'}${p.x},${p.y}`).join(' ');
  return { grid, dots, d };
});
</script>

<template>
  <svg width="100%" :viewBox="`0 0 ${width} ${height}`">
    <template v-for="g in chart.grid" :key="g.y">
      <line :x1="PAD_L" :x2="width - PAD_R" :y1="g.y" :y2="g.y" stroke="#EEEEEE" />
      <text :x="PAD_L - 6" :y="g.y + 4" font-size="11" fill="#757575" text-anchor="end">{{ g.label }}</text>
    </template>
    <path :d="chart.d" fill="none" stroke="#1565C0" stroke-width="2" />
    <template v-for="(p, i) in chart.dots" :key="i">
      <circle :cx="p.x" :cy="p.y" r="3.5" fill="#1565C0"><title>{{ p.title }}</title></circle>
      <text v-if="p.label" :x="p.x" :y="height - 8" font-size="11" fill="#757575" text-anchor="middle">{{ p.label }}</text>
    </template>
  </svg>
</template>
