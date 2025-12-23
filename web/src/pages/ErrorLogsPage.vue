<template>
  <div class="app-page">
    <el-card class="app-card">
      <template #header>
        <div class="card-header">
          <span>{{ t("errorLogs.title") }}</span>
          <el-button @click="refresh">{{ t("actions.refresh") }}</el-button>
        </div>
      </template>

      <el-table :data="paged" stripe v-loading="loading">
        <el-table-column prop="timestamp" :label="t('table.time')" width="170">
          <template #default="scope">{{ formatDateTime(scope.row.timestamp) }}</template>
        </el-table-column>
        <el-table-column prop="level" :label="t('errorLogs.level')" width="90" />
        <el-table-column prop="source" :label="t('errorLogs.source')" width="140" />
        <el-table-column prop="message" :label="t('errorLogs.message')" min-width="360" show-overflow-tooltip />
        <el-table-column :label="t('table.actions')" width="120">
          <template #default="scope">
            <el-button size="small" type="primary" @click="openDetail(scope.row)">{{ t('common.detail') }}</el-button>
          </template>
        </el-table-column>
      </el-table>

      <UnifiedPagination v-model:page="page" v-model:pageSize="pageSize" :total="list.length" />
    </el-card>

    <el-dialog v-model="detailVisible" :title="detailTitle" width="900px" :close-on-click-modal="false">
      <el-descriptions :column="2" border>
        <el-descriptions-item :label="t('table.id')">{{ detail?.id }}</el-descriptions-item>
        <el-descriptions-item :label="t('errorLogs.level')">{{ detail?.level }}</el-descriptions-item>
        <el-descriptions-item :label="t('errorLogs.source')">{{ detail?.source }}</el-descriptions-item>
        <el-descriptions-item :label="t('table.time')">{{ detail ? formatDateTime(detail.timestamp) : '-' }}</el-descriptions-item>
        <el-descriptions-item :label="t('errorLogs.message')" :span="2">{{ detail?.message }}</el-descriptions-item>
        <el-descriptions-item :label="t('errorLogs.detail')" :span="2">
          <pre class="mono">{{ detail?.detail || '-' }}</pre>
        </el-descriptions-item>
      </el-descriptions>

      <el-collapse style="margin-top: 12px">
        <el-collapse-item :title="t('errorLogs.context')" name="context">
          <pre class="mono">{{ detail?.context || '-' }}</pre>
        </el-collapse-item>
        <el-collapse-item :title="t('errorLogs.stack')" name="stack">
          <pre class="mono">{{ detail?.stack || '-' }}</pre>
        </el-collapse-item>
      </el-collapse>

      <template #footer>
        <el-button @click="detailVisible = false">{{ t('common.close') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

import { api } from "@/api/client";
import type { ErrorLog } from "@/api/types";
import UnifiedPagination from "@/components/UnifiedPagination.vue";
import { formatDateTime } from "@/utils/format";

const { t } = useI18n();

const loading = ref(false);
const list = ref<ErrorLog[]>([]);

const page = ref(1);
const pageSize = ref(20);

const paged = computed(() => {
  const start = (page.value - 1) * pageSize.value;
  return list.value.slice(start, start + pageSize.value);
});

async function refresh() {
  loading.value = true;
  try {
    const res = await api.get<ErrorLog[]>("/error-logs");
    list.value = res.data;
  } finally {
    loading.value = false;
  }
}

const detailVisible = ref(false);
const detail = ref<ErrorLog | null>(null);

const detailTitle = computed(() => {
  if (!detail.value) return t("errorLogs.title");
  return `${t("errorLogs.title")} #${detail.value.id}`;
});

function openDetail(row: ErrorLog) {
  detail.value = row;
  detailVisible.value = true;
}

onMounted(() => {
  refresh().catch(() => undefined);
});
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
