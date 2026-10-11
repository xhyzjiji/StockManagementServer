<script setup lang="ts">
// 物品卡片：列表 / 网格 / 缩略图三种视图；多选模式显示勾选框，列表视图带快捷入库。
// Item card for list / grid / thumbnail views; shows a checkbox in multi-select mode and quick stock-in in list view.
import { computed } from 'vue';
import ThumbImage from './ThumbImage.vue';
import TagChip from './TagChip.vue';
import type { ItemView } from './types';
import type { ItemSummary } from '@/api/types';
import { daysSince, fileUrl, fmtMoney, fmtQtyUnit } from '@/utils/format';

const props = defineProps<{ item: ItemSummary; view: ItemView; selecting: boolean; selected: boolean }>();
const emit = defineEmits<{ open: []; toggle: []; quickIn: [] }>();

const priceText = computed(() => (props.item.avg_price > 0 ? `¥${fmtMoney(props.item.avg_price)}/${props.item.unit || '件'}` : '未定价'));
const stockText = computed(() => (props.item.quantity > 0 ? `库存 ${fmtQtyUnit(props.item.quantity, props.item.unit)}` : '暂无库存'));

function onClick() {
  if (props.selecting) emit('toggle');
  else emit('open');
}
</script>

<template>
  <div class="card item-card" :class="{ selected }" @click="onClick">
    <ThumbImage :src="fileUrl(item.first_image)" />
    <div :class="{ grow: view !== 'thumb' }">
      <h3 class="ellipsis">{{ item.name }}</h3>
      <div v-if="item.tags.length" class="tags"><TagChip v-for="t in item.tags.slice(0, 6)" :key="t" :tag="t" /></div>
      <div v-if="view !== 'thumb'" class="meta">
        <div>{{ stockText }} · {{ priceText }}</div>
        <div v-if="item.stock_since">在库 {{ daysSince(item.stock_since) }} 天</div>
        <div v-if="view === 'list' && item.attr_summary" class="ellipsis">{{ item.attr_summary }}</div>
      </div>
    </div>
    <van-checkbox v-if="selecting" class="select-box" :model-value="selected" @click.stop="emit('toggle')" />
    <van-button v-else-if="view === 'list'" class="quick-in" size="small" round plain type="primary" @click.stop="emit('quickIn')">+ 入库</van-button>
  </div>
</template>
