<script setup lang="ts">
// 作品详情：图片、材料成本、品次列表；支持追加数量与用时并重算平均用时。
// Work detail: images, material cost, batch list; append qty/time and recalculate average minutes.
import { computed, onMounted, reactive, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { showImagePreview } from 'vant';
import PageLayout from '@/components/PageLayout.vue';
import LoadState from '@/components/LoadState.vue';
import MoreMenu from '@/components/MoreMenu.vue';
import FormDialog from '@/components/FormDialog.vue';
import type { MenuItem } from '@/components/types';
import { workApi } from '@/api';
import { confirm, showError, toast } from '@/utils/feedback';
import { fileUrl, fmtMinutes, fmtMoney, fmtQty, fmtQtyUnit, fmtTime } from '@/utils/format';
import { usePageData } from '@/utils/loader';

const route = useRoute();
const router = useRouter();
const id = Number(route.params.id);

const { data, loading, notFound, reload } = usePageData(() => workApi.get(id));
const work = computed(() => data.value?.work);

const showAppend = ref(false);
const appendForm = reactive({ quantity: '', minutes: '', note: '' });
const appendErrors = reactive({ quantity: '', minutes: '' });

watch(showAppend, (v) => {
  if (!v) return;
  Object.assign(appendForm, { quantity: '', minutes: '', note: '' });
  Object.assign(appendErrors, { quantity: '', minutes: '' });
});

onMounted(() => {
  if (route.query.append === '1') showAppend.value = true;
});

const menu = computed<MenuItem[]>(() => [
  { text: '编辑', onClick: () => router.push(`/works/${id}/edit`) },
  { text: '追加数量', onClick: () => { showAppend.value = true; } },
  {
    text: '删除', danger: true, onClick: async () => {
      if (!work.value) return;
      if (!(await confirm(`删除作品 "${work.value.name}"？`))) return;
      try {
        await workApi.remove(id);
        toast('已删除');
        router.replace('/works');
      } catch (e) {
        showError(e);
      }
    },
  },
]);

function preview(startPath: string) {
  const imgs = (data.value?.images ?? []).map((i) => fileUrl(i.path));
  const start = Math.max(0, imgs.indexOf(fileUrl(startPath)));
  showImagePreview({ images: imgs, startPosition: start, closeable: true });
}

async function submitAppend(): Promise<boolean> {
  Object.assign(appendErrors, { quantity: '', minutes: '' });
  const q = Number(appendForm.quantity);
  const mins = Number(appendForm.minutes);
  if (!appendForm.quantity.trim() || !Number.isFinite(q) || q <= 0) {
    appendErrors.quantity = '请输入大于 0 的数量';
    return false;
  }
  if (!appendForm.minutes.trim() || !Number.isFinite(mins) || mins < 0) {
    appendErrors.minutes = '请输入制作用时（分钟）';
    return false;
  }
  await workApi.appendBatch(id, { quantity: q, minutes: mins, note: appendForm.note.trim() });
  toast('已追加');
  await reload();
  return true;
}
</script>

<template>
  <PageLayout :title="work?.name ?? '作品详情'" back="/works">
    <template #actions>
      <van-icon name="edit" title="编辑" @click="router.push(`/works/${id}/edit`)" />
      <van-icon name="plus" title="追加数量" @click="showAppend = true" />
      <MoreMenu :items="menu" />
    </template>

    <LoadState :loading="loading" :not-found="notFound">
      <template v-if="data && work">
        <div v-if="data.images.length" class="work-gallery card">
          <img
            v-for="img in data.images" :key="img.id"
            :src="fileUrl(img.path)" :alt="work.name" loading="lazy"
            :class="{ cover: img.is_cover }" @click="preview(img.path)"
          />
        </div>

        <div class="card section">
          <div class="muted small">累计产出</div>
          <div class="qty-big">{{ fmtQty(work.quantity) }} 件</div>
          <div class="muted">单件材料成本 ¥{{ fmtMoney(work.unit_material_cost) }} · 平均 {{ fmtMinutes(work.avg_minutes) }}/件</div>
          <div class="muted small">材料总成本（录入时锁定）¥{{ fmtMoney(work.material_cost) }} · 累计用时 {{ fmtMinutes(work.total_minutes) }}</div>
          <div v-if="work.remark" class="muted" style="margin-top: 8px; white-space: pre-wrap">{{ work.remark }}</div>
          <div class="actions-row">
            <van-button round type="primary" @click="showAppend = true">追加数量 / 用时</van-button>
          </div>
        </div>

        <div class="card section">
          <h2>使用材料</h2>
          <div class="list">
            <div v-for="m in data.materials" :key="m.id" class="kv">
              <div class="k ellipsis">{{ m.item_name }}</div>
              <div class="v">
                {{ fmtQtyUnit(m.quantity, m.unit) }} · ¥{{ fmtMoney(m.unit_price) }}/{{ m.unit || '件' }}
                <div class="muted small">{{ m.warehouse_name }} · 小计 ¥{{ fmtMoney(m.line_cost) }}</div>
              </div>
            </div>
          </div>
        </div>

        <div class="card" style="margin-bottom: 12px">
          <div class="section" style="margin-bottom: 0; padding-bottom: 4px"><h2 style="margin: 0">制作记录</h2></div>
          <div v-if="data.batches.length" class="list">
            <div v-for="b in data.batches" :key="b.id" class="record">
              <span class="badge in">+{{ fmtQty(b.quantity) }} 件</span>
              <div class="grow">
                <div class="small">用时 {{ fmtMinutes(b.minutes) }} · 折合 {{ fmtMinutes(b.minutes / b.quantity) }}/件</div>
                <div v-if="b.note" class="small muted" style="word-break: break-word">{{ b.note }}</div>
              </div>
              <div class="small muted" style="white-space: nowrap">{{ fmtTime(b.created_at) }}</div>
            </div>
          </div>
          <van-empty v-else image-size="64" description="暂无制作记录" />
        </div>
      </template>
    </LoadState>
  </PageLayout>

  <FormDialog v-model:show="showAppend" title="追加数量 / 用时" :submit="submitAppend">
    <p class="msg">追加后会重新计算平均制作用时；材料成本仍按首次录入锁定。</p>
    <van-field v-model="appendForm.quantity" class="cw-field" label-align="top" label="追加数量（件）" inputmode="decimal" placeholder="数量" :error-message="appendErrors.quantity" autofocus />
    <van-field v-model="appendForm.minutes" class="cw-field" label-align="top" label="本次用时（分钟）" inputmode="decimal" placeholder="分钟" :error-message="appendErrors.minutes" />
    <van-field v-model="appendForm.note" class="cw-field" label-align="top" label="备注" maxlength="1000" placeholder="可选" />
  </FormDialog>
</template>
