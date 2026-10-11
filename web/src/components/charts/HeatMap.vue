<script setup lang="ts">
// 出入库频率热力图：days 为本地纪元日 → 笔数；每列一周（周一→周日），最后一列包含今天。
// Activity heatmap: days maps local epoch day → count; one column per week (Mon→Sun), the last column contains today.
import { computed, onMounted, ref } from 'vue';
import { DAY_MS, fmtDayShort } from '@/utils/format';

const props = withDefaults(defineProps<{
  days: Record<string, number>;
  palette: string[];
  weeks?: number;
  tzOffsetMin?: number;
}>(), { weeks: 26, tzOffsetMin: () => -new Date().getTimezoneOffset() });

const CELL = 13;
const GAP = 3;
const LEFT = 22;
const TOP = 18;

const width = computed(() => LEFT + props.weeks * (CELL + GAP));
const height = TOP + 7 * (CELL + GAP);
const weekdayLabels: [number, string][] = [[0, '一'], [2, '三'], [4, '五']];

function levelOf(count: number, max: number): number {
  if (count <= 0 || max <= 0) return 0;
  const r = count / max;
  if (r <= 0.25) return 1;
  if (r <= 0.5) return 2;
  if (r <= 0.75) return 3;
  return 4;
}

const layout = computed(() => {
  const offsetMs = props.tzOffsetMin * 60_000;
  const today = Math.floor((Date.now() + offsetMs) / DAY_MS);
  // 纪元日 0（1970-01-01）是周四；第 0 行为周一
  // Epoch day 0 (1970-01-01) is a Thursday; row 0 is Monday.
  const rowOf = (d: number) => (((d + 3) % 7) + 7) % 7;
  const firstMonday = today - rowOf(today) - (props.weeks - 1) * 7;
  const max = Math.max(0, ...Object.values(props.days).map(Number));
  // 纪元日 → 当地日期中午的时间戳，避免时区边界误差
  // Epoch day → local noon timestamp, avoiding time-zone edge errors.
  const dayToMs = (d: number) => d * DAY_MS - offsetMs + 12 * 3_600_000;
  const cells: { x: number; y: number; fill: string; title: string }[] = [];
  const months: { x: number; label: string }[] = [];
  let lastMonthX = -100;
  let prevMonth = -1;
  for (let w = 0; w < props.weeks; w++) {
    const monday = firstMonday + w * 7;
    const month = new Date(dayToMs(monday)).getMonth();
    const x = LEFT + w * (CELL + GAP);
    if (month !== prevMonth && x - lastMonthX > 28) {
      months.push({ x, label: `${month + 1}月` });
      lastMonthX = x;
    }
    prevMonth = month;
    for (let r = 0; r < 7; r++) {
      const d = monday + r;
      if (d > today) continue;
      const n = Number(props.days[d] ?? 0);
      cells.push({ x, y: TOP + r * (CELL + GAP), fill: props.palette[levelOf(n, max)], title: `${fmtDayShort(dayToMs(d))}：${n} 笔` });
    }
  }
  return { cells, months };
});

// 窄屏时默认滚动到最右侧，优先展示最近几周
// On narrow screens scroll to the right edge so the most recent weeks are visible first.
const scroller = ref<HTMLElement | null>(null);
onMounted(() => {
  if (scroller.value) scroller.value.scrollLeft = scroller.value.scrollWidth;
});
</script>

<template>
  <div ref="scroller" class="chart-scroll">
    <svg :width="width" :height="height" :viewBox="`0 0 ${width} ${height}`" role="img">
      <text v-for="[row, label] in weekdayLabels" :key="row" x="0" :y="TOP + row * (CELL + GAP) + CELL - 2" font-size="10" fill="#757575">{{ label }}</text>
      <text v-for="m in layout.months" :key="m.x" :x="m.x" y="11" font-size="10" fill="#757575">{{ m.label }}</text>
      <rect v-for="c in layout.cells" :key="`${c.x}-${c.y}`" :x="c.x" :y="c.y" :width="CELL" :height="CELL" rx="2" :fill="c.fill">
        <title>{{ c.title }}</title>
      </rect>
    </svg>
  </div>
  <div class="heat-legend">少<i v-for="c in palette" :key="c" :style="{ background: c }" />多</div>
</template>
