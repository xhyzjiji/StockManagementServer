<script setup lang="ts">
// 物品详情：大图、标签、库存与出入库、属性、最近流水。
// Item detail: hero image, tags, stock with in/out actions, attributes and recent records.
import { computed, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { showImagePreview } from 'vant';
import PageLayout from '@/components/PageLayout.vue';
import LoadState from '@/components/LoadState.vue';
import MoreMenu from '@/components/MoreMenu.vue';
import TagChip from '@/components/TagChip.vue';
import RecordRow from '@/components/RecordRow.vue';
import StockDialog from '@/components/StockDialog.vue';
import type { MenuItem } from '@/components/types';
import { itemApi } from '@/api';
import { confirm, showError, toast } from '@/utils/feedback';
import { daysSince, fileUrl, fmtMoney, fmtQtyUnit, splitTags } from '@/utils/format';
import { usePageData } from '@/utils/loader';

const RECENT_LIMIT = 5;
const route = useRoute();
const router = useRouter();
const id = Number(route.params.id);

const { data, loading, notFound, reload } = usePageData(async () => {
  const [detail, records] = await Promise.all([itemApi.get(id), itemApi.records(id, 0, RECENT_LIMIT)]);
  return { detail, records };
});

const item = computed(() => data.value?.detail.item);
const hero = computed(() => data.value?.detail.attrs.find((a) => a.def.type === 'IMAGE' && a.value));
const tags = computed(() => [...new Set((data.value?.detail.attrs ?? []).filter((a) => a.def.type === 'TAGS').flatMap((a) => splitTags(a.value)))]);
const attrRows = computed(() => (data.value?.detail.attrs ?? []).filter((a) => a.def.type !== 'TAGS' && a !== hero.value));

const menu: MenuItem[] = [{ text: '删除', danger: true, onClick: remove }];

async function remove() {
  const d = data.value?.detail;
  if (!d) return;
  if (!(await confirm(`删除物品 "${d.item.name}"？将先移入回收站，照片暂保留，可在回收站恢复或彻底删除。`))) return;
  try {
    await itemApi.removeMany(d.warehouse_id, [d.item.id]);
    toast('已移入回收站');
    router.replace(`/w/${d.warehouse_id}`);
  } catch (e) {
    showError(e);
  }
}

function preview(rel: string) {
  showImagePreview({ images: [fileUrl(rel)], closeable: true });
}

const stock = ref({ show: false, isIn: true });
function openStock(isIn: boolean) {
  stock.value = { show: true, isIn };
}
</script>

<template>
  <PageLayout :title="item?.name ?? '物品详情'" :back="data ? `/w/${data.detail.warehouse_id}` : '/'">
    <template #actions>
      <van-icon name="edit" title="编辑" @click="router.push(`/item/${id}/edit`)" />
      <MoreMenu :items="menu" />
    </template>

    <LoadState :loading="loading" :not-found="notFound">
      <template v-if="data && item">
        <img v-if="hero" class="hero" :src="fileUrl(hero.value)" :alt="item.name" @click="preview(hero.value)" />

        <div class="card section">
          <div v-if="tags.length" class="tags" style="margin-bottom: 12px"><TagChip v-for="t in tags" :key="t" :tag="t" /></div>
          <div class="muted small">当前库存</div>
          <div class="qty-big">{{ fmtQtyUnit(item.quantity, item.unit) }}</div>
          <div class="muted">均价 ¥{{ fmtMoney(item.avg_price) }} · 剩余总价 ¥{{ fmtMoney(item.total_value) }}</div>
          <div v-if="item.stock_since" class="muted small">在库 {{ daysSince(item.stock_since) }} 天</div>
          <div class="actions-row">
            <van-button round color="var(--cw-in)" @click="openStock(true)">入库</van-button>
            <van-button round color="var(--cw-out)" @click="openStock(false)">出库</van-button>
          </div>
        </div>

        <div v-if="attrRows.length" class="card section">
          <h2>属性</h2>
          <div class="list">
            <div v-for="a in attrRows" :key="a.def.id" class="kv">
              <div class="k">{{ a.def.name }}</div>
              <div class="v">
                <template v-if="a.def.type === 'IMAGE'">
                  <img v-if="a.value" :src="fileUrl(a.value)" :alt="a.def.name" loading="lazy" @click="preview(a.value)" />
                  <span v-else class="muted">—</span>
                </template>
                <template v-else>
                  <template v-if="a.value">{{ a.value }}</template>
                  <span v-else class="muted">—</span>
                </template>
              </div>
            </div>
          </div>
        </div>

        <div class="card" style="margin-bottom: 12px">
          <div class="section" style="margin-bottom: 0; padding-bottom: 4px"><h2 style="margin: 0">最近出入库</h2></div>
          <div v-if="data.records.list.length" class="list">
            <RecordRow v-for="r in data.records.list" :key="r.id" :record="r" />
          </div>
          <van-empty v-else image-size="64" description="暂无出入库记录" />
          <div v-if="data.records.total > data.records.list.length" style="padding: 8px 16px 12px">
            <van-button block round plain hairline type="primary" @click="router.push(`/item/${id}/records`)">
              查看全部（共 {{ data.records.total }} 条）
            </van-button>
          </div>
        </div>

        <StockDialog v-model:show="stock.show" :item="item" :is-in="stock.isIn" @done="reload" />
      </template>
    </LoadState>
  </PageLayout>
</template>
