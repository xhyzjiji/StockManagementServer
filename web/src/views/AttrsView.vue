<script setup lang="ts">
// 属性设置：仓库级自定义属性（全仓库物品共用），支持增删改与排序，整体保存；有未保存修改时离开需确认。
// Attribute settings: warehouse-level custom attributes shared by all items; add/edit/delete/reorder, saved as a whole.
// Leaving with unsaved changes asks for confirmation.
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import { onBeforeRouteLeave, useRoute } from 'vue-router';
import PageLayout from '@/components/PageLayout.vue';
import LoadState from '@/components/LoadState.vue';
import AttrDialog from '@/components/AttrDialog.vue';
import type { AttrDraftRow } from '@/components/types';
import { attrApi, warehouseApi } from '@/api';
import type { AttrType, AttributeDef } from '@/api/types';
import { confirm, showError, toast } from '@/utils/feedback';
import { usePageData } from '@/utils/loader';

const TYPE_NAMES: Record<AttrType, string> = { TEXT: '文本', NUMBER: '数字', CHOICE: '单选', TAGS: '标签', IMAGE: '图片' };

const whID = Number(useRoute().params.id);
const drafts = ref<AttrDraftRow[]>([]);
const dirty = ref(false);
const saving = ref(false);

const toDrafts = (defs: AttributeDef[]): AttrDraftRow[] =>
  defs.map((d) => ({ id: d.id, name: d.name, type: d.type, options: d.options, originalType: d.type }));

const { data: wh, loading, notFound } = usePageData(async () => {
  const [w, defs] = await Promise.all([warehouseApi.get(whID), attrApi.list(whID)]);
  drafts.value = toDrafts(defs);
  return w;
});

const dialog = reactive({
  show: false,
  title: '',
  index: -1,
  draft: { id: null, name: '', type: 'TEXT', options: [], originalType: null } as AttrDraftRow,
  existingNames: [] as string[],
});

function openCreate() {
  Object.assign(dialog, {
    show: true, title: '新建属性', index: -1,
    draft: { id: null, name: '', type: 'TEXT', options: [], originalType: null },
    existingNames: drafts.value.map((d) => d.name),
  });
}

function openEdit(i: number) {
  Object.assign(dialog, {
    show: true, title: '编辑属性', index: i, draft: drafts.value[i],
    existingNames: drafts.value.filter((_, j) => j !== i).map((d) => d.name),
  });
}

function onDialogOk(d: AttrDraftRow) {
  if (dialog.index >= 0) drafts.value[dialog.index] = d;
  else drafts.value.push(d);
  dirty.value = true;
}

async function remove(i: number) {
  const d = drafts.value[i];
  const warn = d.id ? `删除属性 "${d.name}"？保存后该仓库下所有物品此属性的值和参考图将被清除。` : `删除属性 "${d.name}"？`;
  if (!(await confirm(warn))) return;
  drafts.value.splice(i, 1);
  dirty.value = true;
}

function move(i: number, delta: number) {
  const list = drafts.value;
  [list[i], list[i + delta]] = [list[i + delta], list[i]];
  dirty.value = true;
}

async function save() {
  saving.value = true;
  try {
    const saved = await attrApi.save(whID, drafts.value.map(({ id, name, type, options }) => ({ id, name, type, options })));
    drafts.value = toDrafts(saved);
    dirty.value = false;
    toast('已保存');
  } catch (e) {
    showError(e);
  } finally {
    saving.value = false;
  }
}

onBeforeRouteLeave(async () => !dirty.value || confirm('属性设置还没有保存，确定离开？', { okText: '离开' }));

const onBeforeUnload = (e: BeforeUnloadEvent) => {
  if (dirty.value) e.preventDefault();
};
onMounted(() => window.addEventListener('beforeunload', onBeforeUnload));
onBeforeUnmount(() => window.removeEventListener('beforeunload', onBeforeUnload));
</script>

<template>
  <PageLayout title="属性设置" :subtitle="wh?.name" :back="`/w/${whID}`">
    <LoadState :loading="loading" :not-found="notFound">
      <p class="muted small hint">属性对仓库内所有物品生效；修改后点击底部「保存属性设置」才会生效。</p>
      <div class="card" style="margin-bottom: 12px">
        <van-empty v-if="!drafts.length" class="pre-line" description="该仓库还没有属性&#10;点击下方按钮添加（属性对仓库内所有物品生效）" />
        <div v-else class="list">
          <div v-for="(d, i) in drafts" :key="`${d.id ?? 'new'}-${i}`" class="attr-row">
            <div class="grow">
              <div class="row"><b class="ellipsis">{{ d.name }}</b><span class="type-badge">{{ TYPE_NAMES[d.type] }}</span></div>
              <div v-if="d.options.length" class="muted small ellipsis">{{ d.options.join('、') }}</div>
            </div>
            <van-button size="small" plain hairline :disabled="i === 0" @click="move(i, -1)">↑</van-button>
            <van-button size="small" plain hairline :disabled="i === drafts.length - 1" @click="move(i, 1)">↓</van-button>
            <van-button size="small" plain hairline type="primary" @click="openEdit(i)">编辑</van-button>
            <van-button size="small" plain hairline type="danger" @click="remove(i)">删除</van-button>
          </div>
        </div>
      </div>
      <div class="row" style="gap: 12px">
        <van-button class="grow" round plain type="primary" @click="openCreate">＋ 添加属性</van-button>
        <van-button class="grow" round type="primary" :loading="saving" @click="save">
          {{ dirty ? '保存属性设置（有未保存的修改）' : '保存属性设置' }}
        </van-button>
      </div>
    </LoadState>
  </PageLayout>

  <AttrDialog v-model:show="dialog.show" :title="dialog.title" :draft="dialog.draft" :existing-names="dialog.existingNames" @ok="onDialogOk" />
</template>
