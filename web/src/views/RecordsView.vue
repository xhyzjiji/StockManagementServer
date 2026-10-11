<script setup lang="ts">
// 出入库流水：仓库级（scope=warehouse）或物品级（scope=item），滚动到底自动加载下一页。
// Stock records for a warehouse (scope=warehouse) or an item (scope=item); the next page loads when scrolled to the bottom.
import { onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import PageLayout from '@/components/PageLayout.vue';
import RecordRow from '@/components/RecordRow.vue';
import { itemApi, warehouseApi } from '@/api';
import { ApiError } from '@/api/http';
import type { Page, StockRecord } from '@/api/types';
import { showError } from '@/utils/feedback';

const PAGE_SIZE = 30;
const props = defineProps<{ scope: 'warehouse' | 'item' }>();
const id = Number(useRoute().params.id);

const subtitle = ref('');
const records = ref<StockRecord[]>([]);
const total = ref(0);
const loading = ref(false);
const finished = ref(false);
const failed = ref(false);
const notFound = ref(false);

const back = props.scope === 'warehouse' ? `/w/${id}` : `/item/${id}`;
const fetchPage = (offset: number): Promise<Page<StockRecord>> =>
  props.scope === 'warehouse' ? warehouseApi.records(id, offset, PAGE_SIZE) : itemApi.records(id, offset, PAGE_SIZE);

onMounted(async () => {
  try {
    subtitle.value = props.scope === 'warehouse' ? (await warehouseApi.get(id)).name : (await itemApi.get(id)).item.name;
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) notFound.value = true;
    else showError(e);
  }
});

async function onLoad() {
  try {
    const res = await fetchPage(records.value.length);
    records.value.push(...res.list);
    total.value = res.total;
    finished.value = records.value.length >= res.total || !res.list.length;
  } catch (e) {
    failed.value = true;
    showError(e);
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <PageLayout title="出入库记录" :subtitle="subtitle" :back="back">
    <van-empty v-if="notFound" class="card" image="error" description="页面不存在或已被删除" />
    <div v-else class="card">
      <van-list
        v-model:loading="loading" v-model:error="failed" :finished="finished"
        :finished-text="total ? `共 ${total} 条` : ''" error-text="加载失败，点击重试" @load="onLoad"
      >
        <div class="list"><RecordRow v-for="r in records" :key="r.id" :record="r" :show-item-name="scope === 'warehouse'" /></div>
      </van-list>
      <van-empty v-if="finished && !total" description="暂无出入库记录" />
    </div>
  </PageLayout>
</template>
