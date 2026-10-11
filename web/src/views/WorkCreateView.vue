<script setup lang="ts">
// 作品多步录入/编辑：图片（选封面）→ 选材料 → 填用量算成本；创建时另有数量与用时步。
// Multi-step work create/edit: photos (cover) → materials → usages & cost; create also has qty/time.
import { computed, onMounted, reactive, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { showImagePreview } from 'vant';
import PageLayout from '@/components/PageLayout.vue';
import ThumbImage from '@/components/ThumbImage.vue';
import { materialApi, uploadApi, warehouseApi, workApi } from '@/api';
import type { MaterialPick, Warehouse, WorkInput, WorkUpdateInput } from '@/api/types';
import { showError, toast } from '@/utils/feedback';
import { fileUrl, fmtMinutes, fmtMoney, fmtQtyUnit } from '@/utils/format';
import { compressImage, pickFile } from '@/utils/image';

interface StagedImg {
  path: string;
  /** 已落盘的作品图（编辑保留） / existing committed work image (kept on edit) */
  existing?: boolean;
  uploading?: boolean;
}

interface MatLine {
  pick: MaterialPick;
  qty: string;
}

const route = useRoute();
const router = useRouter();
const editingID = computed(() => {
  const raw = route.params.id;
  return typeof raw === 'string' && /^\d+$/.test(raw) ? Number(raw) : 0;
});
const isEdit = computed(() => editingID.value > 0);

const step = ref(0);
const steps = computed(() =>
  isEdit.value ? ['图片', '选材料', '用量'] : ['图片', '选材料', '用量', '数量用时']);
const lastStep = computed(() => steps.value.length - 1);
const saving = ref(false);
const bootLoading = ref(false);

const form = reactive({ name: '', remark: '' });
const nameError = ref('');
const images = ref<StagedImg[]>([]);
const coverPath = ref('');
const warehouses = ref<Warehouse[]>([]);
const warehouseID = ref(0);
const materials = ref<MaterialPick[]>([]);
const selected = ref(new Map<number, MaterialPick>());
const lines = ref<MatLine[]>([]);
const matSearch = ref('');
const matLoading = ref(false);
const whLoading = ref(false);
const batch = reactive({ quantity: '1', minutes: '', note: '' });
const batchErrors = reactive({ quantity: '', minutes: '' });

const materialCost = computed(() => {
  let sum = 0;
  for (const line of lines.value) {
    const q = Number(line.qty);
    if (Number.isFinite(q) && q > 0) sum += q * line.pick.avg_price;
  }
  return Math.round(sum * 100 + 1e-7) / 100;
});

const workQty = computed(() => Number(batch.quantity));
const unitCost = computed(() => {
  const q = workQty.value;
  if (!Number.isFinite(q) || q <= 0) return 0;
  return Math.round((materialCost.value / q) * 100 + 1e-7) / 100;
});

const selectedList = computed(() => [...selected.value.values()]);
const selectedInWarehouse = computed(() =>
  selectedList.value.filter((m) => m.warehouse_id === warehouseID.value).length);

const pageTitle = computed(() => (isEdit.value ? '编辑作品' : '录入作品'));
const backTo = computed(() => (isEdit.value ? `/works/${editingID.value}` : '/works'));

async function loadWarehouses() {
  whLoading.value = true;
  try {
    warehouses.value = await warehouseApi.list();
    if (!warehouseID.value && warehouses.value.length) {
      warehouseID.value = warehouses.value[0].id;
    }
  } catch (e) {
    showError(e);
  } finally {
    whLoading.value = false;
  }
}

async function loadMaterials() {
  if (!warehouseID.value) {
    materials.value = [];
    return;
  }
  matLoading.value = true;
  try {
    materials.value = await materialApi.list(warehouseID.value, matSearch.value.trim());
    // 用当前仓库列表刷新已选材料的均价/库存展示
    // Refresh selected picks' avg/stock from the current warehouse list.
    const next = new Map(selected.value);
    for (const m of materials.value) {
      if (next.has(m.item_id)) next.set(m.item_id, m);
    }
    selected.value = next;
    lines.value = lines.value.map((l) => {
      const pick = next.get(l.pick.item_id);
      return pick ? { ...l, pick } : l;
    });
  } catch (e) {
    showError(e);
  } finally {
    matLoading.value = false;
  }
}

async function loadEdit() {
  if (!isEdit.value) return;
  bootLoading.value = true;
  try {
    const d = await workApi.get(editingID.value);
    form.name = d.work.name;
    form.remark = d.work.remark;
    images.value = d.images.map((img) => ({ path: img.path, existing: true }));
    coverPath.value = d.images.find((i) => i.is_cover)?.path || d.work.cover_image || images.value[0]?.path || '';
    const map = new Map<number, MaterialPick>();
    for (const m of d.materials) {
      map.set(m.item_id, {
        item_id: m.item_id,
        item_name: m.item_name,
        unit: m.unit,
        quantity: 0,
        avg_price: m.unit_price,
        warehouse_id: m.warehouse_id,
        warehouse_name: m.warehouse_name,
        first_image: '',
      });
    }
    selected.value = map;
    lines.value = d.materials.map((m) => ({
      pick: map.get(m.item_id)!,
      qty: String(m.quantity),
    }));
    if (d.materials.length) {
      warehouseID.value = d.materials[0].warehouse_id;
    }
  } catch (e) {
    showError(e);
    router.replace('/works');
  } finally {
    bootLoading.value = false;
  }
}

onMounted(async () => {
  await loadWarehouses();
  await loadEdit();
});

// 切换仓库时清空搜索并重新加载该仓库材料；已选材料跨仓库保留
// Switching warehouse clears search and reloads that warehouse's items; prior selections are kept.
watch(warehouseID, () => {
  matSearch.value = '';
  loadMaterials();
});

let searchTimer = 0;
function onSearch() {
  clearTimeout(searchTimer);
  searchTimer = window.setTimeout(loadMaterials, 300);
}

function selectWarehouse(id: number) {
  if (warehouseID.value !== id) warehouseID.value = id;
}

function toggleMaterial(m: MaterialPick) {
  const next = new Map(selected.value);
  if (next.has(m.item_id)) next.delete(m.item_id);
  else next.set(m.item_id, m);
  selected.value = next;
}

function removeSelected(itemId: number) {
  const next = new Map(selected.value);
  next.delete(itemId);
  selected.value = next;
}

async function addImage() {
  if (images.value.length >= 20) {
    toast('最多上传 20 张图片');
    return;
  }
  const file = await pickFile('image/*');
  if (!file) return;
  const placeholder: StagedImg = { path: '', uploading: true };
  images.value = [...images.value, placeholder];
  try {
    const res = await uploadApi.image(await compressImage(file));
    placeholder.path = res.path;
    placeholder.uploading = false;
    if (!coverPath.value) coverPath.value = res.path;
    images.value = [...images.value];
  } catch (e) {
    images.value = images.value.filter((x) => x !== placeholder);
    showError(e);
  }
}

function removeImage(path: string) {
  images.value = images.value.filter((x) => x.path !== path);
  if (coverPath.value === path) coverPath.value = images.value[0]?.path || '';
}

function preview(path: string) {
  const urls = images.value.filter((x) => x.path).map((x) => fileUrl(x.path));
  const start = Math.max(0, urls.indexOf(fileUrl(path)));
  showImagePreview({ images: urls, startPosition: start, closeable: true });
}

function buildImageInputs() {
  return images.value.map((img) => {
    const is_cover = img.path === coverPath.value;
    if (img.existing) return { path: img.path, is_cover };
    return { staged_image: img.path, is_cover };
  });
}

// 回退上一步；表单与已选数据保留，不离开页面。
// Go to previous wizard step; keep form/selection without leaving the page.
function goPrev() {
  if (saving.value || step.value <= 0) return;
  step.value -= 1;
}

// 顶部返回：非第一步则回退向导，否则离开页面。
// Nav back: previous wizard step when not on first; otherwise leave the page.
function onNavBack() {
  if (step.value > 0) {
    goPrev();
    return;
  }
  if (window.history.state?.back) router.back();
  else router.replace(backTo.value);
}

// 点击已完成步骤可直接跳回（不可跳到未到达步）。
// Jump back by tapping a completed step indicator (not forward to unreached steps).
function jumpToStep(i: number) {
  if (saving.value || i < 0 || i >= step.value) return;
  step.value = i;
}

function goNext() {
  if (step.value === 0) {
    nameError.value = '';
    if (!form.name.trim()) {
      nameError.value = '请输入作品名称';
      return;
    }
    if (!images.value.length || images.value.some((x) => x.uploading)) {
      toast(images.value.some((x) => x.uploading) ? '图片还在上传，请稍候' : '请至少上传一张作品图片');
      return;
    }
    if (!coverPath.value || !images.value.some((x) => x.path === coverPath.value)) {
      toast('请选择封面缩略图');
      return;
    }
  }
  if (step.value === 1) {
    if (!selected.value.size) {
      toast('请至少选择一种材料');
      return;
    }
    // 进入用量步：保留已填用量，新增材料默认空
    // Enter usage step: keep filled usages; new materials start empty.
    const prev = new Map(lines.value.map((l) => [l.pick.item_id, l.qty]));
    lines.value = [...selected.value.values()].map((pick) => ({
      pick, qty: prev.get(pick.item_id) || '',
    }));
  }
  if (step.value === 2) {
    for (const line of lines.value) {
      const q = Number(line.qty);
      if (!line.qty.trim() || !Number.isFinite(q) || q <= 0) {
        toast(`请填写「${line.pick.item_name}」的用量`);
        return;
      }
    }
    // 编辑模式：用量步即最后一步，直接保存
    // Edit mode: usage is the last step — save immediately.
    if (isEdit.value) {
      submitEdit();
      return;
    }
  }
  step.value += 1;
}

async function submitEdit() {
  const body: WorkUpdateInput = {
    name: form.name.trim(),
    remark: form.remark.trim(),
    images: buildImageInputs(),
    materials: lines.value.map((l) => ({ item_id: l.pick.item_id, quantity: Number(l.qty) })),
  };
  saving.value = true;
  try {
    await workApi.update(editingID.value, body);
    toast('已保存');
    router.replace(`/works/${editingID.value}`);
  } catch (e) {
    showError(e);
  } finally {
    saving.value = false;
  }
}

async function submit() {
  batchErrors.quantity = '';
  batchErrors.minutes = '';
  const q = Number(batch.quantity);
  const mins = Number(batch.minutes);
  if (!batch.quantity.trim() || !Number.isFinite(q) || q <= 0) {
    batchErrors.quantity = '请输入大于 0 的数量';
    return;
  }
  if (!batch.minutes.trim() || !Number.isFinite(mins) || mins < 0) {
    batchErrors.minutes = '请输入制作用时（分钟）';
    return;
  }
  const body: WorkInput = {
    name: form.name.trim(),
    remark: form.remark.trim(),
    images: buildImageInputs(),
    materials: lines.value.map((l) => ({ item_id: l.pick.item_id, quantity: Number(l.qty) })),
    quantity: q,
    minutes: mins,
    note: batch.note.trim(),
  };
  saving.value = true;
  try {
    const res = await workApi.create(body);
    toast('作品已录入');
    router.replace(`/works/${res.id}`);
  } catch (e) {
    showError(e);
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <PageLayout :title="pageTitle" :back="backTo" :back-action="onNavBack">
    <div v-if="bootLoading" style="padding: 48px; text-align: center"><van-loading /></div>
    <template v-else>
      <div class="work-steps card">
        <button
          v-for="(label, i) in steps" :key="label" type="button" class="work-step"
          :class="{ active: i === step, done: i < step, jump: i < step }"
          :disabled="i >= step" @click="jumpToStep(i)"
        >
          <span class="n">{{ i + 1 }}</span>{{ label }}
        </button>
      </div>

      <!-- 步骤 1：名称与图片 / Step 1: name & photos -->
      <template v-if="step === 0">
        <div class="card section">
          <van-field v-model="form.name" class="cw-field" label-align="top" label="作品名称" maxlength="255" placeholder="如：手工托特包" :error-message="nameError" />
          <van-field v-model="form.remark" class="cw-field" label-align="top" label="备注" type="textarea" rows="2" autosize maxlength="1000" placeholder="可选" />
        </div>
        <div class="card section">
          <h2>作品图片</h2>
          <p class="muted small hint">上传后点选一张作为列表封面缩略图</p>
          <div class="work-imgs">
            <div v-for="img in images" :key="img.path || 'up'" class="work-img" :class="{ cover: img.path && img.path === coverPath }">
              <div v-if="img.uploading" class="thumb"><van-loading /></div>
              <template v-else>
                <ThumbImage :src="fileUrl(img.path)" @click="preview(img.path)" />
                <button type="button" class="cover-btn" @click="coverPath = img.path">{{ img.path === coverPath ? '封面' : '设为封面' }}</button>
                <button type="button" class="rm-btn" @click="removeImage(img.path)">×</button>
              </template>
            </div>
            <button type="button" class="work-img add" @click="addImage"><van-icon name="plus" /><span>添加</span></button>
          </div>
        </div>
      </template>

      <!-- 步骤 2：先选仓库，再在该仓库内搜索/勾选材料；可切换仓库继续多选 -->
      <!-- Step 2: pick a warehouse first, then search/select items in it; switch warehouses to accumulate. -->
      <template v-else-if="step === 1">
        <div class="card section">
          <h2>选择仓库</h2>
          <div v-if="whLoading" style="padding: 16px; text-align: center"><van-loading /></div>
          <van-empty v-else-if="!warehouses.length" image-size="64" description="还没有仓库，请先去「我的仓库」创建" />
          <div v-else class="chips">
            <div
              v-for="w in warehouses" :key="w.id" class="chip"
              :class="{ active: warehouseID === w.id }" @click="selectWarehouse(w.id)"
            >
              {{ w.name }}
            </div>
          </div>
        </div>

        <div v-if="selectedList.length" class="card section">
          <div class="row" style="margin-bottom: 8px">
            <h2 style="margin: 0" class="grow">已选 {{ selectedList.length }} 种</h2>
            <span class="muted small">可切换仓库继续添加</span>
          </div>
          <div class="chips wrap">
            <div v-for="m in selectedList" :key="m.item_id" class="chip active" @click="removeSelected(m.item_id)">
              {{ m.item_name }} · {{ m.warehouse_name }} ×
            </div>
          </div>
        </div>

        <template v-if="warehouseID">
          <div class="card section">
            <van-search
              v-model="matSearch" shape="round"
              placeholder="在当前仓库中搜索材料" @update:model-value="onSearch"
            />
            <p class="muted small" style="margin: 8px 0 0">
              当前仓库已选 {{ selectedInWarehouse }} 种
            </p>
          </div>
          <div v-if="matLoading" style="padding: 32px; text-align: center"><van-loading /></div>
          <van-empty v-else-if="!materials.length" class="card" :description="matSearch ? '当前仓库没有匹配的材料' : '当前仓库暂无物品'" />
          <div v-else class="card" style="margin-bottom: 12px">
            <div
              v-for="m in materials" :key="m.item_id" class="mat-row"
              :class="{ on: selected.has(m.item_id) }" @click="toggleMaterial(m)"
            >
              <ThumbImage class="mat-thumb" :src="fileUrl(m.first_image)" />
              <div class="grow">
                <b class="ellipsis">{{ m.item_name }}</b>
                <div class="muted small">库存 {{ fmtQtyUnit(m.quantity, m.unit) }} · 均价 ¥{{ fmtMoney(m.avg_price) }}</div>
              </div>
              <van-checkbox :model-value="selected.has(m.item_id)" @click.stop="toggleMaterial(m)" />
            </div>
          </div>
        </template>
      </template>

      <!-- 步骤 3：用量与成本 / Step 3: usages & cost -->
      <template v-else-if="step === 2">
        <div class="card section">
          <h2>材料用量</h2>
          <p class="muted small hint">
            {{ isEdit
              ? '按配方用量填写；保存后按当前仓库均价重新锁定成本（不扣减库存）。累计件数与用时请用「追加」。'
              : '按本次制作实际用量填写；成本按仓库当前均价计算（不扣减库存）' }}
          </p>
          <div v-for="line in lines" :key="line.pick.item_id" class="usage-row">
            <div class="grow">
              <b>{{ line.pick.item_name }}</b>
              <div class="muted small">{{ line.pick.warehouse_name }} · ¥{{ fmtMoney(line.pick.avg_price) }}/{{ line.pick.unit || '件' }}</div>
            </div>
            <van-field
              v-model="line.qty" class="cw-field usage-qty" label-align="top"
              :label="line.pick.unit ? `用量（${line.pick.unit}）` : '用量'"
              inputmode="decimal" placeholder="0"
            />
          </div>
          <div class="cost-bar">材料成本合计 <b>¥{{ fmtMoney(materialCost) }}</b></div>
        </div>
      </template>

      <!-- 步骤 4：数量与用时（仅创建） / Step 4: quantity & time (create only) -->
      <template v-else>
        <div class="card section">
          <h2>本次制作</h2>
          <van-field
            v-model="batch.quantity" class="cw-field" label-align="top" label="作品数量（件）"
            inputmode="decimal" placeholder="1" :error-message="batchErrors.quantity"
          />
          <van-field
            v-model="batch.minutes" class="cw-field" label-align="top" label="制作用时（分钟）"
            inputmode="decimal" placeholder="如：120" :error-message="batchErrors.minutes"
          />
          <van-field v-model="batch.note" class="cw-field" label-align="top" label="备注" maxlength="1000" placeholder="可选" />
          <div class="cost-bar">
            <div>材料总成本 ¥{{ fmtMoney(materialCost) }}</div>
            <div>单件材料成本 <b>¥{{ fmtMoney(unitCost) }}</b></div>
            <div v-if="Number(batch.minutes) >= 0 && workQty > 0" class="muted small" style="margin-top: 4px">
              平均用时约 {{ fmtMinutes(Number(batch.minutes) / workQty) }}/件
            </div>
          </div>
        </div>
      </template>

      <div class="work-nav">
        <van-button v-if="step > 0" round plain type="primary" :disabled="saving" @click="goPrev">上一步</van-button>
        <van-button v-if="step < lastStep" class="grow" round type="primary" :disabled="saving" @click="goNext">下一步</van-button>
        <van-button
          v-else class="grow" round type="primary" :loading="saving"
          @click="isEdit ? goNext() : submit()"
        >
          {{ isEdit ? '保存' : '提交录入' }}
        </van-button>
      </div>
    </template>
  </PageLayout>
</template>
