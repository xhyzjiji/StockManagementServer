<script setup lang="ts">
// 回收站列表：恢复或彻底删除（彻底删除前二次确认）。
// Recycle bin list: restore, or permanently delete after confirmation.
import type { DeletedRow } from '@/api/types';
import { confirm, showError, toast } from '@/utils/feedback';
import { fmtTime } from '@/utils/format';

const props = defineProps<{
  rows: DeletedRow[];
  destroyMessage: (row: DeletedRow) => string;
  onRestore: (row: DeletedRow) => Promise<unknown>;
  onDestroy: (row: DeletedRow) => Promise<unknown>;
}>();

async function restore(row: DeletedRow) {
  try {
    await props.onRestore(row);
    toast('已恢复');
  } catch (e) {
    showError(e);
  }
}

async function destroy(row: DeletedRow) {
  if (!(await confirm(props.destroyMessage(row), { okText: '彻底删除' }))) return;
  try {
    await props.onDestroy(row);
    toast('已删除');
  } catch (e) {
    showError(e);
  }
}
</script>

<template>
  <van-empty v-if="!rows.length" class="card" description="回收站是空的" />
  <template v-else>
    <p class="muted small hint">恢复后数据原样回到原位置；彻底删除会永久清除数据与照片，不可恢复。</p>
    <div class="card list">
      <div v-for="r in rows" :key="r.id" class="attr-row">
        <div class="grow">
          <b class="ellipsis" style="display: block">{{ r.title }}</b>
          <div class="muted small ellipsis">{{ [r.subtitle, `删除于 ${fmtTime(r.deleted_at)}`].filter(Boolean).join(' · ') }}</div>
        </div>
        <van-button size="small" type="primary" plain round @click="restore(r)">恢复</van-button>
        <van-button size="small" type="danger" plain round @click="destroy(r)">彻底删除</van-button>
      </div>
    </div>
  </template>
</template>
