<script setup lang="ts">
// 入库 / 出库弹窗：入库必填总价（最多两位小数）、可选购买时间；老库存未定价时先选择计价方式。
// Stock in/out dialog: inbound requires a total price (≤ 2 decimals) and an optional purchase time;
// unpriced existing stock asks for a pricing mode first.
import { computed, reactive, watch } from 'vue';
import FormDialog from './FormDialog.vue';
import { itemApi } from '@/api';
import { ApiError } from '@/api/http';
import type { Item, Pricing, StockInput } from '@/api/types';
import { pickOption, promptText } from '@/utils/dialogs';
import { toast } from '@/utils/feedback';
import { PRICE_RE, fmtQtyUnit, toLocalInput } from '@/utils/format';

type StockItem = Pick<Item, 'id' | 'name' | 'unit' | 'quantity' | 'total_value'>;

const props = defineProps<{ item: StockItem; isIn: boolean }>();
const show = defineModel<boolean>('show', { required: true });
const emit = defineEmits<{ done: [] }>();

const form = reactive({ qty: '', price: '', time: '', note: '' });
const errors = reactive({ qty: '', price: '', time: '' });
const maxTime = computed(() => (show.value ? toLocalInput(Date.now()) : ''));

watch(show, (v) => {
  if (!v) return;
  Object.assign(form, { qty: '', price: '', time: '', note: '' });
  Object.assign(errors, { qty: '', price: '', time: '' });
});

// choosePricing 返回计价方式与修正总值；取消返回 null。
// choosePricing resolves to the pricing mode and corrected value, or null when cancelled.
async function choosePricing(): Promise<{ pricing: Pricing; corrected_value?: number } | null> {
  const pick = await pickOption(
    '老库存尚未定价',
    ['按首次定价（整批库存按本次单价）', '忽略老库存价格（本次总价摊到全部库存）', '手动修正老库存总价'],
    `当前库存 ${fmtQtyUnit(props.item.quantity, props.item.unit)} 还没有价格，请选择本次入库的计价方式`,
  );
  if (pick === 0) return { pricing: 'FIRST_PRICE' };
  if (pick === 1) return { pricing: 'WEIGHTED' };
  if (pick !== 2) return null;
  const v = await promptText({
    title: '手动修正老库存总价', label: '老库存总价（元）', placeholder: '0.00', inputmode: 'decimal',
    validate: (s) => (PRICE_RE.test(s) ? null : '请输入金额，最多两位小数'),
  });
  return v === null ? null : { pricing: 'CORRECT', corrected_value: Number(v) };
}

async function submit(): Promise<boolean> {
  Object.assign(errors, { qty: '', price: '', time: '' });
  const { item, isIn } = props;
  const rawQty = form.qty.trim();
  const q = Number(rawQty);
  if (!rawQty || !Number.isFinite(q) || q <= 0) {
    errors.qty = '请输入大于 0 的数量';
    return false;
  }
  if (!isIn && q > item.quantity + 1e-9) {
    errors.qty = `库存不足：当前 ${fmtQtyUnit(item.quantity, item.unit)}`;
    return false;
  }
  const body: StockInput = { type: isIn ? 'IN' : 'OUT', quantity: q, note: form.note.trim() };
  if (isIn) {
    const rawPrice = form.price.trim();
    if (!PRICE_RE.test(rawPrice)) {
      errors.price = '请输入本次入库总价，最多两位小数';
      return false;
    }
    body.total_price = Number(rawPrice);
    if (form.time) {
      const at = new Date(form.time).getTime();
      if (!Number.isFinite(at) || at > Date.now()) {
        errors.time = '购买时间不能晚于当前时间';
        return false;
      }
      body.occurred_at = at;
    }
    if (item.quantity > 0 && item.total_value <= 0) {
      const p = await choosePricing();
      if (!p) return false;
      Object.assign(body, p);
    }
  }
  try {
    await itemApi.stock(item.id, body);
  } catch (e) {
    if (e instanceof ApiError && e.status === 409) {
      const available = typeof e.data.available === 'number' ? e.data.available : item.quantity;
      errors.qty = `库存不足：当前 ${fmtQtyUnit(available, item.unit)}`;
      return false;
    }
    throw e;
  }
  toast(isIn ? '入库成功' : '出库成功');
  emit('done');
  return true;
}
</script>

<template>
  <FormDialog v-model:show="show" :title="`${isIn ? '入库' : '出库'} · ${item.name}`" :submit="submit">
    <p class="msg">当前库存：{{ fmtQtyUnit(item.quantity, item.unit) }}</p>
    <van-field
      v-model="form.qty" class="cw-field" label-align="top" :label="item.unit ? `数量（${item.unit}）` : '数量'"
      inputmode="decimal" placeholder="数量" :error-message="errors.qty" autofocus
    />
    <template v-if="isIn">
      <van-field
        v-model="form.price" class="cw-field" label-align="top" label="本次入库总价（元）"
        inputmode="decimal" placeholder="0.00" :error-message="errors.price"
      />
      <van-field class="cw-field" label-align="top" label="购买时间（可选，默认当前时间）" :error-message="errors.time">
        <template #input>
          <input v-model="form.time" class="native-input" type="datetime-local" :max="maxTime" />
        </template>
      </van-field>
    </template>
    <van-field v-model="form.note" class="cw-field" label-align="top" label="备注" maxlength="1000" placeholder="备注（可选）" />
  </FormDialog>
</template>
