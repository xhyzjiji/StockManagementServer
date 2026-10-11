<script setup lang="ts">
import PageLayout from '@/components/PageLayout.vue';
</script>

<template>
  <PageLayout title="页面不存在" back="/">
    <van-empty class="card" image="error" description="页面不存在或已被删除" />
  </PageLayout>
</template>
