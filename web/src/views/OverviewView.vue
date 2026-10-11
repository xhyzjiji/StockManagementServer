<script setup lang="ts">
// 仓库主页：物品总览（类目筛选 + 按名称/标签搜索 + 排序 + 三种视图 + 多选删除 + 快捷入库）。
// 筛选条件保存在地址栏 query 中，从详情页返回时自动恢复。
// Warehouse home: item overview (category filter, name/tag search, sorting, three views, multi-select delete,
// quick stock-in). Filters live in the URL query so they are restored when coming back from a detail page.
import { computed, reactive, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import PageLayout from '@/components/PageLayout.vue';
import LoadState from '@/components/LoadState.vue';
import MoreMenu from '@/components/MoreMenu.vue';
import ItemCard from '@/components/ItemCard.vue';
import StockDialog from '@/components/StockDialog.vue';
import WarehouseDialog from '@/components/WarehouseDialog.vue';
import type { ItemView, MenuItem } from '@/components/types';
import { categoryApi, itemApi, warehouseApi } from '@/api';
import type { Category, ItemQuery, ItemSummary } from '@/api/types';
import { pickOption, promptText } from '@/utils/dialogs';
import { confirm, showError, toast } from '@/utils/feedback';
import { usePageData } from '@/utils/loader';
import { exportWarehouse } from '@/utils/transfer';

const VIEW_KEY = 'cw_item_view';
type Sort = NonNullable<ItemQuery['sort']>;

const route = useRoute();
const router = useRouter();
const whID = Number(route.params.id);

const str = (v: unknown) => (typeof v === 'string' ? v : '');
const filters = reactive({
  category: Number(route.query.c) || 0,
  q: str(route.query.q),
  by: (route.query.by === 'tag' ? 'tag' : 'name') as 'name' | 'tag',
  sort: (['days_desc', 'days_asc'].includes(str(route.query.sort)) ? str(route.query.sort) : '') as Sort,
});
const view = ref<ItemView>((['list', 'grid', 'thumb'] as const).find((v) => v === localStorage.getItem(VIEW_KEY)) ?? 'list');
const categories = ref<Category[]>([]);
const items = ref<ItemSummary[]>([]);
const itemsLoading = ref(true);
const selecting = ref(false);
const selected = ref(new Set<number>());

const { data: wh, loading, notFound } = usePageData(async () => {
  const [w, cats] = await Promise.all([warehouseApi.get(whID), categoryApi.list(whID)]);
  categories.value = cats;
  if (filters.category && !cats.some((c) => c.id === filters.category)) filters.category = 0;
  await loadItems();
  return w;
});

const byOptions = [{ text: '按名称', value: 'name' }, { text: '按标签', value: 'tag' }];
const sortOptions = [
  { text: '录入时间（新→旧）', value: '' },
  { text: '在库天数（长→短）', value: 'days_desc' },
  { text: '在库天数（短→长）', value: 'days_asc' },
];
const viewOptions = [{ text: '列表', value: 'list' }, { text: '网格', value: 'grid' }, { text: '缩略图', value: 'thumb' }];

const emptyText = computed(() => {
  if (filters.q) return '没有找到匹配的物品';
  return categories.value.length ? '暂无物品\n点击右下角按钮添加物品' : '还没有类目\n先点击上方「＋ 新建类目」';
});

let loadSeq = 0;
async function loadItems() {
  const seq = ++loadSeq;
  try {
    const list = await itemApi.list(whID, { category_id: filters.category, q: filters.q.trim(), by: filters.by, sort: filters.sort });
    if (seq !== loadSeq) return;
    items.value = list;
    const ids = new Set(list.map((it) => it.id));
    selected.value = new Set([...selected.value].filter((id) => ids.has(id)));
  } catch (e) {
    showError(e);
  } finally {
    if (seq === loadSeq) itemsLoading.value = false;
  }
}

async function reloadCategories() {
  categories.value = await categoryApi.list(whID);
  if (filters.category && !categories.value.some((c) => c.id === filters.category)) filters.category = 0;
}

// 筛选变化：写回地址栏（不重建页面）并重新加载；关键字输入做防抖
// Filter changes are written back to the URL (without rebuilding the page) and reload the list; keyword input is debounced.
let qTimer = 0;
watch(() => [filters.category, filters.by, filters.sort, filters.q] as const, ([c, by, sort, q], [, oldBy, , oldQ]) => {
  router.replace({ query: { c: c || undefined, q: q || undefined, by: by === 'tag' ? 'tag' : undefined, sort: sort || undefined } });
  clearTimeout(qTimer);
  if (q !== oldQ) qTimer = window.setTimeout(loadItems, 300);
  else if (by !== oldBy && !q) return;
  else loadItems();
});

watch(view, (v) => localStorage.setItem(VIEW_KEY, v));

const menu = computed<MenuItem[]>(() => [
  { text: '属性设置', onClick: () => router.push(`/w/${whID}/attrs`) },
  { text: '出入库记录', onClick: () => router.push(`/w/${whID}/records`) },
  { text: '数据统计', onClick: () => router.push(`/w/${whID}/stats`) },
  { text: selecting.value ? '退出多选' : '多选删除', onClick: () => setSelecting(!selecting.value) },
  { text: '物品回收站', onClick: () => router.push(`/w/${whID}/bin`) },
  { text: '仓库信息', onClick: () => { whDialog.value = true; } },
  { text: '导出数据包', onClick: () => exportWarehouse(whID) },
]);

const whDialog = ref(false);
async function saveWarehouse(name: string, remark: string) {
  await warehouseApi.update(whID, name, remark);
  if (wh.value) Object.assign(wh.value, { name, remark });
  toast('已保存');
}

function selectCategory(id: number) {
  filters.category = id;
}

async function categoryActions(cat: Category) {
  const pick = await pickOption(cat.name, ['重命名', '删除']);
  if (pick === 0) {
    await promptText({
      title: '重命名', label: '类目名称', value: cat.name, maxlength: 128,
      onOk: async (name) => {
        await categoryApi.rename(cat.id, name);
        await reloadCategories();
      },
    });
  } else if (pick === 1) {
    if (!(await confirm(`删除类目 "${cat.name}"？其中的物品将全部删除！`))) return;
    try {
      await categoryApi.remove(cat.id);
      filters.category = 0;
      await reloadCategories();
    } catch (e) {
      showError(e);
    }
  }
}

async function createCategory() {
  await promptText({
    title: '新建类目', label: '类目名称', placeholder: '如：饮料、零食', maxlength: 128,
    onOk: async (name) => {
      const res = await categoryApi.create(whID, name);
      await reloadCategories();
      filters.category = res.id;
    },
  });
}

function toggleSelect(id: number) {
  const next = new Set(selected.value);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  selected.value = next;
}

function setSelecting(on: boolean) {
  selecting.value = on;
  selected.value = new Set();
}

async function deleteSelected() {
  const ids = [...selected.value];
  if (!ids.length) return;
  if (!(await confirm(`将选中的 ${ids.length} 个物品移入回收站？照片暂保留，可在回收站恢复或彻底删除。`))) return;
  try {
    await itemApi.removeMany(whID, ids);
    toast('已移入回收站');
    setSelecting(false);
    await Promise.all([loadItems(), reloadCategories()]);
  } catch (e) {
    showError(e);
  }
}

async function addItem() {
  if (filters.category) {
    router.push({ path: '/item/new', query: { category: filters.category } });
    return;
  }
  if (!categories.value.length) {
    toast('请先新建一个类目');
    return;
  }
  const pick = await pickOption('添加到哪个类目？', categories.value.map((c) => c.name));
  if (pick >= 0) router.push({ path: '/item/new', query: { category: categories.value[pick].id } });
}

const stock = reactive({ show: false, item: null as ItemSummary | null });
function quickIn(it: ItemSummary) {
  stock.item = it;
  stock.show = true;
}
async function onStockDone() {
  await loadItems();
}
</script>

<template>
  <PageLayout :title="wh?.name ?? '仓库'" :subtitle="wh?.remark || undefined" back="/">
    <template #actions>
      <van-icon name="bar-chart-o" title="数据统计" @click="router.push(`/w/${whID}/stats`)" />
      <MoreMenu :items="menu" />
    </template>

    <LoadState :loading="loading" :not-found="notFound">
      <div class="toolbar">
        <van-search v-model="filters.q" shape="round" :placeholder="filters.by === 'tag' ? '输入标签名' : '输入物品名称'" />
        <van-dropdown-menu>
          <van-dropdown-item v-model="filters.by" :options="byOptions" />
          <van-dropdown-item v-model="filters.sort" :options="sortOptions" />
          <van-dropdown-item v-model="view" :options="viewOptions" />
        </van-dropdown-menu>
      </div>

      <div class="chips">
        <div class="chip" :class="{ active: filters.category === 0 }" @click="selectCategory(0)">全部</div>
        <div
          v-for="c in categories" :key="c.id" class="chip" :class="{ active: filters.category === c.id }"
          @click="filters.category === c.id ? categoryActions(c) : selectCategory(c.id)"
        >
          {{ c.name }} · {{ c.item_count }}
          <span v-if="filters.category === c.id" class="chip-more">⋯</span>
        </div>
        <div class="chip dashed" @click="createCategory">＋ 新建类目</div>
      </div>

      <div v-if="itemsLoading" style="padding: 32px 0; text-align: center"><van-loading /></div>
      <van-empty v-else-if="!items.length" class="card pre-line" :description="emptyText" />
      <div v-else class="items" :class="`${view}-view`">
        <ItemCard
          v-for="it in items" :key="it.id" :item="it" :view="view" :selecting="selecting" :selected="selected.has(it.id)"
          @open="router.push(`/item/${it.id}`)" @toggle="toggleSelect(it.id)" @quick-in="quickIn(it)"
        />
      </div>
    </LoadState>

    <template #footer>
      <div v-if="selecting" class="bottom-bar">
        <span class="grow">已选 {{ selected.size }} 项</span>
        <van-button size="small" round @click="setSelecting(false)">取消</van-button>
        <van-button size="small" round type="danger" :disabled="!selected.size" @click="deleteSelected">移入回收站</van-button>
      </div>
      <button v-else-if="!loading && !notFound" class="fab" @click="addItem"><van-icon name="plus" /><span class="fab-label">添加物品</span></button>
    </template>
  </PageLayout>

  <StockDialog v-if="stock.item" v-model:show="stock.show" :item="stock.item" :is-in="true" @done="onStockDone" />
  <WarehouseDialog v-model:show="whDialog" title="仓库信息" :name="wh?.name" :remark="wh?.remark" :on-ok="saveWarehouse" />
</template>
