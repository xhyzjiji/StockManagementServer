<script setup lang="ts">
// 页面骨架：顶部导航（标题 / 副标题 / 返回 / 右侧操作）+ 内容区 + 可选底部区。
// Page shell: top nav (title / subtitle / back / right actions), content area and an optional footer.
import type { RouteLocationRaw } from 'vue-router';
import { useRouter } from 'vue-router';

const props = defineProps<{
  title: string;
  subtitle?: string;
  back?: RouteLocationRaw;
  /** 自定义返回（如向导回退上一步）；提供时覆盖默认 history/fallback。 / Custom back (e.g. wizard prev step); overrides default. */
  backAction?: () => void;
}>();
const router = useRouter();

// 有站内历史时后退（保留上一页的筛选与滚动），否则跳到兜底页
// Go back when there is in-app history (keeping the previous page state), otherwise go to the fallback.
function goBack() {
  if (props.backAction) {
    props.backAction();
    return;
  }
  if (window.history.state?.back) router.back();
  else if (props.back) router.replace(props.back);
}
</script>

<template>
  <div>
    <van-nav-bar class="cw-nav" fixed placeholder safe-area-inset-top :left-arrow="!!back" @click-left="goBack">
      <template #title>
        <div class="nav-title">
          <div class="ellipsis">{{ title }}</div>
          <div v-if="subtitle" class="nav-sub ellipsis">{{ subtitle }}</div>
        </div>
      </template>
      <template #right>
        <div class="nav-actions"><slot name="actions" /></div>
      </template>
    </van-nav-bar>
    <main class="container"><slot /></main>
    <slot name="footer" />
  </div>
</template>
