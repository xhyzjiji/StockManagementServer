<script setup lang="ts">
// 仓库回收站：全部已删除仓库。
// Warehouse recycle bin: all deleted warehouses.
import PageLayout from '@/components/PageLayout.vue';
import LoadState from '@/components/LoadState.vue';
import RecycleList from '@/components/RecycleList.vue';
import { warehouseApi } from '@/api';
import type { DeletedRow } from '@/api/types';
import { usePageData } from '@/utils/loader';

const { data: rows, loading, reload } = usePageData(() => warehouseApi.listDeleted());

const destroyMessage = (r: DeletedRow) => `彻底删除仓库 "${r.title}"？其中的类目、物品、流水与照片将永久删除，不可恢复！`;
const restore = async (r: DeletedRow) => { await warehouseApi.restore(r.id); await reload(); };
const destroy = async (r: DeletedRow) => { await warehouseApi.destroy(r.id); await reload(); };
</script>

<template>
  <PageLayout title="仓库回收站" back="/warehouses">
    <LoadState :loading="loading">
      <RecycleList :rows="rows ?? []" :destroy-message="destroyMessage" :on-restore="restore" :on-destroy="destroy" />
    </LoadState>
  </PageLayout>
</template>
