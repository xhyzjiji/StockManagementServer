<script setup lang="ts">
// 单条出入库流水；仓库级列表展示物品名，物品已删除时给出提示。
// One stock record; warehouse-level lists show the item name and flag deleted items.
import type { StockRecord } from '@/api/types';
import { fmtMoney, fmtQty, fmtQtyUnit, fmtTime } from '@/utils/format';

defineProps<{ record: StockRecord; showItemName?: boolean }>();
</script>

<template>
  <div class="record">
    <span class="badge" :class="record.type === 'IN' ? 'in' : 'out'">
      {{ record.type === 'IN' ? '入库 +' : '出库 -' }}{{ fmtQty(record.quantity) }}
    </span>
    <div class="grow">
      <div v-if="showItemName" class="ellipsis">
        <router-link v-if="record.item_exists" :to="`/item/${record.item_id}`" style="font-weight: 600">{{ record.item_name }}</router-link>
        <template v-else><b>{{ record.item_name }}</b><span class="muted small">（物品已删除）</span></template>
      </div>
      <div class="small">
        {{ fmtQty(record.before_qty) }} → {{ fmtQtyUnit(record.after_qty, record.unit) }}
        <template v-if="record.total_price !== null"> · 金额 ¥{{ fmtMoney(record.total_price) }}</template>
      </div>
      <div v-if="record.note" class="small muted" style="word-break: break-word">{{ record.note }}</div>
    </div>
    <div class="small muted" style="white-space: nowrap">{{ fmtTime(record.created_at) }}</div>
  </div>
</template>
