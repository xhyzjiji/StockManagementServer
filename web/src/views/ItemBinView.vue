<script setup lang="ts">
// 物品回收站：指定仓库的已删除物品。
// Item recycle bin: deleted items of one warehouse.
import { useRoute } from 'vue-router';
import PageLayout from '@/components/PageLayout.vue';
import LoadState from '@/components/LoadState.vue';
import RecycleList from '@/components/RecycleList.vue';
import { itemApi, warehouseApi } from '@/api';
import type { DeletedRow } from '@/api/types';
import { usePageData } from '@/utils/loader';

const whID = Number(useRoute().params.id);
const { data, loading, notFound, reload } = usePageData(async () => {
  const [wh, rows] = await Promise.all([warehouseApi.get(whID), itemApi.listDeleted(whID)]);
  return { wh, rows };
});

const destroyMessage = (r: DeletedRow) => `彻底删除物品 "${r.title}"？其参考图将永久删除，不可恢复！`;
const restore = async (r: DeletedRow) => { await itemApi.restore(r.id); await reload(); };
const destroy = async (r: DeletedRow) => { await itemApi.destroy(r.id); await reload(); };
</script>

<template>
  <PageLayout title="物品回收站" :subtitle="data?.wh.name" :back="`/w/${whID}`">
    <LoadState :loading="loading" :not-found="notFound">
      <RecycleList :rows="data?.rows ?? []" :destroy-message="destroyMessage" :on-restore="restore" :on-destroy="destroy" />
    </LoadState>
  </PageLayout>
</template>
