<script setup lang="ts">
// 物品新建 / 编辑：名称、单位、类目，以及按属性类型生成的输入控件（图片先上传到暂存区，保存时才落盘）。
// Item create / edit: name, unit, category and per-type attribute inputs
// (images are uploaded to a staging area and committed only on save).
import { computed, reactive, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { showImagePreview } from 'vant';
import PageLayout from '@/components/PageLayout.vue';
import LoadState from '@/components/LoadState.vue';
import SelectField from '@/components/SelectField.vue';
import ThumbImage from '@/components/ThumbImage.vue';
import { attrApi, categoryApi, itemApi, uploadApi } from '@/api';
import type { AttrValueInput, AttributeDef, Category, ItemInput } from '@/api/types';
import { showError, toast } from '@/utils/feedback';
import { fileUrl, splitTags } from '@/utils/format';
import { compressImage, pickFile } from '@/utils/image';
import { usePageData } from '@/utils/loader';

interface ImageState {
  existing: string;
  staged: string;
  remove: boolean;
  uploading: boolean;
}

const route = useRoute();
const router = useRouter();
const editingID = route.params.id ? Number(route.params.id) : 0;

const form = reactive({ name: '', unit: '件', categoryID: 0, nameError: '' });
const values = reactive<Record<number, string>>({});
const tagSets = reactive<Record<number, string[]>>({});
const images = reactive<Record<number, ImageState>>({});
const defs = ref<AttributeDef[]>([]);
const categories = ref<Category[]>([]);
const whID = ref(0);
const saving = ref(false);

const { loading, notFound } = usePageData(async () => {
  let attrValues: Record<number, string> = {};
  if (editingID) {
    const d = await itemApi.get(editingID);
    Object.assign(form, { name: d.item.name, unit: d.item.unit, categoryID: d.item.category_id });
    whID.value = d.warehouse_id;
    defs.value = d.attrs.map((a) => a.def);
    attrValues = Object.fromEntries(d.attrs.map((a) => [a.def.id, a.value]));
  } else {
    form.categoryID = Number(route.query.category) || 0;
    whID.value = (await categoryApi.get(form.categoryID)).warehouse_id;
    defs.value = await attrApi.list(whID.value);
  }
  categories.value = await categoryApi.list(whID.value);
  for (const def of defs.value) {
    const v = attrValues[def.id] ?? '';
    if (def.type === 'TAGS') tagSets[def.id] = splitTags(v);
    else if (def.type === 'IMAGE') images[def.id] = { existing: v, staged: '', remove: false, uploading: false };
    else values[def.id] = v;
  }
  return true;
});

const categoryOptions = computed(() => categories.value.map((c) => ({ text: c.name, value: c.id })));

// 单选：保留已失效的旧值作为候选，避免编辑时被静默清空
// Choice: keep a stale old value as an option so editing does not silently clear it.
function choiceOptions(def: AttributeDef) {
  const v = values[def.id];
  const opts = !v || def.options.includes(v) ? def.options : [...def.options, v];
  return [{ text: '未选择', value: '' }, ...opts.map((o) => ({ text: o, value: o }))];
}

// 标签：候选值 + 物品上已有但已不在候选中的值
// Tags: the options plus values already on the item that are no longer options.
function tagOptions(def: AttributeDef) {
  return [...new Set([...def.options, ...(tagSets[def.id] ?? [])])];
}

function toggleTag(def: AttributeDef, tag: string) {
  const set = tagSets[def.id];
  const i = set.indexOf(tag);
  if (i >= 0) set.splice(i, 1);
  else set.push(tag);
}

function imageUrl(st: ImageState): string {
  if (st.staged) return fileUrl(st.staged);
  return !st.remove && st.existing ? fileUrl(st.existing) : '';
}

async function pickImage(st: ImageState) {
  const file = await pickFile('image/*');
  if (!file) return;
  st.uploading = true;
  try {
    const res = await uploadApi.image(await compressImage(file));
    st.staged = res.path;
    st.remove = false;
  } catch (e) {
    showError(e);
  } finally {
    st.uploading = false;
  }
}

function clearImage(st: ImageState) {
  st.staged = '';
  st.remove = true;
}

function preview(url: string) {
  if (url) showImagePreview({ images: [url], closeable: true });
}

function collectValues(): AttrValueInput[] {
  return defs.value.map((def) => {
    if (def.type === 'IMAGE') {
      const st = images[def.id];
      return { def_id: def.id, staged_image: st.staged, remove_image: st.remove && !st.staged };
    }
    if (def.type === 'TAGS') {
      const chosen = tagSets[def.id];
      return { def_id: def.id, value: tagOptions(def).filter((o) => chosen.includes(o)).join('|') };
    }
    return { def_id: def.id, value: (values[def.id] ?? '').trim() };
  });
}

async function save() {
  const name = form.name.trim();
  form.nameError = name ? '' : '请输入物品名称';
  if (!name) return;
  if (Object.values(images).some((st) => st.uploading)) {
    toast('图片还在上传，请稍候');
    return;
  }
  const body: ItemInput = { category_id: form.categoryID, name, unit: form.unit.trim(), values: collectValues() };
  saving.value = true;
  try {
    if (editingID) {
      await itemApi.update(editingID, body);
      toast('已保存');
      if (window.history.state?.back) router.back();
      else router.replace(`/item/${editingID}`);
    } else {
      const res = await itemApi.create(body);
      toast('已保存');
      router.replace(`/item/${res.id}`);
    }
  } catch (e) {
    showError(e);
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <PageLayout :title="editingID ? '编辑物品' : '新建物品'" :back="editingID ? `/item/${editingID}` : `/w/${whID}`">
    <LoadState :loading="loading" :not-found="notFound">
      <form @submit.prevent="save">
        <div class="card section">
          <van-field v-model="form.name" class="cw-field" label-align="top" label="物品名称" maxlength="255" placeholder="物品名称（如：可乐）" :error-message="form.nameError" />
          <van-field v-model="form.unit" class="cw-field" label-align="top" label="计量单位" maxlength="32" placeholder="计量单位（如：件、米、㎡、公斤）" />
          <SelectField v-model="form.categoryID" label="所属类目" :options="categoryOptions" />
        </div>

        <div class="card section">
          <h2>属性</h2>
          <p v-if="!defs.length" class="muted">该仓库还没有属性，可在仓库菜单「属性设置」中统一定义（对仓库内所有物品生效）。</p>
          <template v-for="def in defs" :key="def.id">
            <van-field
              v-if="def.type === 'TEXT'" v-model="values[def.id]" class="cw-field" label-align="top"
              :label="def.name" maxlength="2000" placeholder="属性值"
            />
            <van-field
              v-else-if="def.type === 'NUMBER'" v-model="values[def.id]" class="cw-field" label-align="top"
              :label="def.name" inputmode="decimal" placeholder="数字"
            />
            <SelectField v-else-if="def.type === 'CHOICE'" v-model="values[def.id]" :label="def.name" :options="choiceOptions(def)" placeholder="未选择" />
            <div v-else-if="def.type === 'TAGS'" style="margin-bottom: 12px">
              <div class="muted small" style="margin-bottom: 6px">{{ def.name }}（可多选）</div>
              <div class="chips wrap">
                <div v-for="o in tagOptions(def)" :key="o" class="chip" :class="{ active: tagSets[def.id].includes(o) }" @click="toggleTag(def, o)">
                  {{ tagSets[def.id].includes(o) ? '✓ ' : '' }}{{ o }}
                </div>
              </div>
            </div>
            <div v-else-if="def.type === 'IMAGE'" style="margin-bottom: 12px">
              <div class="muted small" style="margin-bottom: 6px">{{ def.name }}</div>
              <div class="img-edit">
                <ThumbImage :src="imageUrl(images[def.id])" @click="preview(imageUrl(images[def.id]))" />
                <div class="row wrap">
                  <van-button size="small" round plain type="primary" native-type="button" :loading="images[def.id].uploading" @click="pickImage(images[def.id])">
                    {{ imageUrl(images[def.id]) ? '更换图片' : '选择图片' }}
                  </van-button>
                  <van-button v-if="imageUrl(images[def.id])" size="small" round plain type="danger" native-type="button" @click="clearImage(images[def.id])">清除</van-button>
                </div>
              </div>
            </div>
          </template>
        </div>

        <van-button type="primary" round block native-type="submit" :loading="saving">保存</van-button>
      </form>
    </LoadState>
  </PageLayout>
</template>
