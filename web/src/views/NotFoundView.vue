<script setup lang="ts">
import PageLayout from '@/components/PageLayout.vue';
</script>

<template>
  <PageLayout title="页面不存在" back="/">
    <van-empty class="card" image="error" description="页面不存在或已被删除">
      <van-button type="primary" round style="margin-top: 8px" @click="$router.push('/')">回到首页</van-button>
    </van-empty>
  </PageLayout>
</template>
