// usePageData 页面首屏数据加载：区分加载中、不存在（404）与其它错误。
// usePageData loads a page's initial data, distinguishing loading, not found (404) and other errors.
import { onMounted, ref, type Ref } from 'vue';
import { ApiError } from '@/api/http';
import { showError } from './feedback';

export function usePageData<T>(loader: () => Promise<T>) {
  const data = ref<T | null>(null) as Ref<T | null>;
  const loading = ref(true);
  const notFound = ref(false);

  async function reload(): Promise<void> {
    try {
      data.value = await loader();
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) notFound.value = true;
      else showError(e);
    } finally {
      loading.value = false;
    }
  }

  onMounted(reload);
  return { data, loading, notFound, reload };
}
