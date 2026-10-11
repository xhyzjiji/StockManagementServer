<script setup lang="ts">
// 创作灵感：占位页，功能开发中。
// Inspiration: placeholder while the feature is under development.
import AppShell from '@/components/AppShell.vue';
import { useRouter } from 'vue-router';

const router = useRouter();
</script>

<template>
  <AppShell>
    <div class="card section coming">
      <div class="coming-icon" style="background: #FFF3E0; color: #E65100"><van-icon name="bulb-o" /></div>
      <h2>创作灵感</h2>
      <p class="muted">收集灵感碎片，随手记录想法与素材。</p>
      <p class="muted small">正在开发中，敬请期待</p>
      <van-button type="primary" round plain style="margin-top: 16px" @click="router.push('/')">返回首页</van-button>
    </div>
  </AppShell>
</template>
