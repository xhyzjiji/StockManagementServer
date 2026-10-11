<script setup lang="ts">
// 数据统计：概览 / 出入库频率热力图 / 标签类别分布饼图 / 物品单价变化折线。
// Stats: overview, activity heatmaps, tag distribution pie chart and item price line chart.
import { computed, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import PageLayout from '@/components/PageLayout.vue';
import LoadState from '@/components/LoadState.vue';
import SelectField from '@/components/SelectField.vue';
import HeatMap from '@/components/charts/HeatMap.vue';
import PieChart from '@/components/charts/PieChart.vue';
import LineChart from '@/components/charts/LineChart.vue';
import { GREENS, PIE_COLORS, REDS } from '@/components/charts/palette';
import { itemApi, warehouseApi } from '@/api';
import type { PricePoint, TagSlice } from '@/api/types';
import { showError } from '@/utils/feedback';
import { fmtMoney, fmtQty, fmtQtyUnit } from '@/utils/format';
import { usePageData } from '@/utils/loader';

const WEEKS = 26;
const whID = Number(useRoute().params.id);
const tz = -new Date().getTimezoneOffset();

const { data, loading, notFound } = usePageData(async () => {
  const [wh, stats, items] = await Promise.all([warehouseApi.get(whID), warehouseApi.stats(whID, tz, WEEKS), itemApi.list(whID)]);
  tagDefID.value = stats.tag_defs[0]?.id ?? 0;
  priceItemID.value = items[0]?.id ?? 0;
  return { wh, stats, items };
});

// ---------- 标签分布 / Tag distribution ----------
const tagDefID = ref(0);
const byItem = ref(false);
const slices = ref<TagSlice[] | null>(null);
const tagDefOptions = computed(() => (data.value?.stats.tag_defs ?? []).map((d) => ({ text: d.name, value: d.id })));
const sliceTotal = computed(() => (slices.value ?? []).reduce((a, b) => a + b.quantity, 0));

let tagSeq = 0;
watch([tagDefID, byItem], async ([defID, group]) => {
  if (!defID) return;
  const seq = ++tagSeq;
  try {
    const res = await warehouseApi.tagStats(whID, defID, group);
    if (seq === tagSeq) slices.value = res;
  } catch (e) {
    showError(e);
  }
});

function sliceText(sl: TagSlice): string {
  const qty = sl.unit ? fmtQtyUnit(sl.quantity, sl.unit) : fmtQty(sl.quantity);
  const pct = sliceTotal.value > 0 ? (sl.quantity / sliceTotal.value) * 100 : 0;
  return `${sl.label} · ${qty} · ${pct.toFixed(1)}%`;
}

// ---------- 单价变化 / Price changes ----------
const priceItemID = ref(0);
const points = ref<PricePoint[] | null>(null);
const itemOptions = computed(() => (data.value?.items ?? []).map((it) => ({ text: it.name, value: it.id })));

let priceSeq = 0;
watch(priceItemID, async (id) => {
  if (!id) return;
  const seq = ++priceSeq;
  try {
    const res = await itemApi.prices(id);
    if (seq === priceSeq) points.value = res;
  } catch (e) {
    showError(e);
  }
});
</script>

<template>
  <PageLayout title="数据统计" :subtitle="data?.wh.name" :back="`/w/${whID}`">
    <LoadState :loading="loading" :not-found="notFound">
      <template v-if="data">
        <div class="stat-cards">
          <div class="card stat-card"><div class="label">物品种类</div><div class="value">{{ data.stats.item_kinds }} 种</div></div>
          <div class="card stat-card"><div class="label">库存总价</div><div class="value">¥{{ fmtMoney(data.stats.total_value) }}</div></div>
        </div>

        <div class="card section">
          <h2>出入库频率（近 {{ WEEKS }} 周）</h2>
          <div class="small muted">入库频率</div>
          <HeatMap :days="data.stats.in_days" :palette="GREENS" :weeks="WEEKS" :tz-offset-min="tz" />
          <div class="small muted" style="margin-top: 12px">出库频率</div>
          <HeatMap :days="data.stats.out_days" :palette="REDS" :weeks="WEEKS" :tz-offset-min="tz" />
        </div>

        <div class="card section">
          <h2>标签类别分布</h2>
          <van-empty v-if="!tagDefOptions.length" image-size="64" description="暂无标签字段" />
          <template v-else>
            <div class="row wrap" style="gap: 16px; margin-bottom: 12px">
              <div style="min-width: 180px"><SelectField v-model="tagDefID" :options="tagDefOptions" plain /></div>
              <van-checkbox v-model="byItem" shape="square">按物品分组</van-checkbox>
            </div>
            <van-empty v-if="slices && !slices.length" image-size="64" description="该标签字段下暂无库存" />
            <div v-else-if="slices" class="pie-wrap">
              <PieChart :slices="slices.map((s) => ({ label: s.label, value: s.quantity }))" />
              <div class="legend grow">
                <div v-for="(sl, i) in slices" :key="sl.label + i" class="row">
                  <span class="dot" :style="{ background: PIE_COLORS[i % PIE_COLORS.length] }" />
                  <span class="ellipsis">{{ sliceText(sl) }}</span>
                </div>
              </div>
            </div>
          </template>
        </div>

        <div class="card section">
          <h2>物品单价变化</h2>
          <van-empty v-if="!itemOptions.length" image-size="64" description="暂无物品" />
          <template v-else>
            <div style="max-width: 320px"><SelectField v-model="priceItemID" :options="itemOptions" plain /></div>
            <div style="margin-top: 12px">
              <van-empty v-if="points && !points.length" image-size="64" description="该物品暂无入库定价记录" />
              <LineChart v-else-if="points" :points="points" />
            </div>
          </template>
        </div>
      </template>
    </LoadState>
  </PageLayout>
</template>
