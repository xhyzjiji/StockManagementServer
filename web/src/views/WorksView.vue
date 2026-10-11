<script setup lang="ts">
// 我的作品：列表展示（类似仓库卡片），支持新建、进入详情、删除。
// My works: card list (similar to warehouses); create, open detail, delete.
import { useRouter } from 'vue-router';
import AppShell from '@/components/AppShell.vue';
import LoadState from '@/components/LoadState.vue';
import MoreMenu from '@/components/MoreMenu.vue';
import ThumbImage from '@/components/ThumbImage.vue';
import type { MenuItem } from '@/components/types';
import { workApi } from '@/api';
import type { Work } from '@/api/types';
import { confirm, showError, toast } from '@/utils/feedback';
import { fileUrl, fmtMinutes, fmtMoney, fmtQty } from '@/utils/format';
import { usePageData } from '@/utils/loader';

const router = useRouter();
const { data: list, loading, reload } = usePageData(() => workApi.list());

function cardMenu(w: Work): MenuItem[] {
  return [
    { text: '编辑', onClick: () => router.push(`/works/${w.id}/edit`) },
    { text: '追加数量', onClick: () => router.push(`/works/${w.id}?append=1`) },
    {
      text: '删除', danger: true, onClick: async () => {
        if (!(await confirm(`删除作品 "${w.name}"？`))) return;
        try {
          await workApi.remove(w.id);
          toast('已删除');
          await reload();
        } catch (e) {
          showError(e);
        }
      },
    },
  ];
}
</script>

<template>
  <AppShell>
    <LoadState :loading="loading">
      <van-empty v-if="!list?.length" class="card pre-line" description="暂无作品&#10;点击右下角按钮录入你的第一个作品" />
      <div v-else class="wh-grid">
        <div v-for="w in list" :key="w.id" class="card wh-card work-card" @click="router.push(`/works/${w.id}`)">
          <ThumbImage class="work-cover" :src="fileUrl(w.cover_image)" />
          <div class="grow">
            <h3 class="ellipsis">{{ w.name }}</h3>
            <div class="muted small ellipsis">{{ w.remark || `共 ${fmtQty(w.quantity)} 件` }}</div>
            <div class="muted small">
              单件成本 ¥{{ fmtMoney(w.unit_material_cost) }} · 平均 {{ fmtMinutes(w.avg_minutes) }}/件
            </div>
          </div>
          <div class="more" @click.stop><MoreMenu :items="cardMenu(w)" /></div>
        </div>
      </div>
    </LoadState>

    <template #footer>
      <button class="fab" @click="router.push('/works/new')"><van-icon name="plus" /><span class="fab-label">录入作品</span></button>
    </template>
  </AppShell>
</template>
