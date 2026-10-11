<script setup lang="ts">
defineProps<{ loading: boolean; notFound?: boolean }>();
</script>

<template>
  <div v-if="loading" style="padding: 48px 0; text-align: center"><van-loading /></div>
  <van-empty v-else-if="notFound" class="card" image="error" description="页面不存在或已被删除" />
  <slot v-else />
</template>
