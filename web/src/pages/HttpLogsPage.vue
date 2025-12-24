<template>
  <div class="app-page">
    <el-card class="app-card">
      <template #header>
        <div class="card-header">
          <span>{{ t("httpLogs.title") }}</span>
          <div class="filters">
            <el-input v-model="q" :placeholder="t('httpLogs.q')" clearable style="width: 240px" />
            <el-button @click="search">{{ t("common.search") }}</el-button>
            <el-button @click="reset">{{ t("common.reset") }}</el-button>
          </div>
        </div>
      </template>

      <el-table :data="rows" stripe v-loading="loading">
        <el-table-column prop="timestamp" :label="t('table.time')" width="170">
          <template #default="scope">{{ formatDateTime(scope.row.timestamp) }}</template>
        </el-table-column>
        <el-table-column prop="mapping_id" :label="t('httpLogs.columns.mapping')" min-width="160" />
        <el-table-column prop="local_port" :label="t('httpLogs.columns.port')" width="90" />
        <el-table-column prop="method" :label="t('httpLogs.columns.method')" width="90" />
        <el-table-column prop="status_code" :label="t('httpLogs.columns.status')" width="90" />
        <el-table-column prop="host" :label="t('httpLogs.columns.host')" min-width="160" />
        <el-table-column prop="url" :label="t('httpLogs.columns.url')" min-width="260" show-overflow-tooltip />
        <el-table-column prop="duration_ms" :label="t('httpLogs.columns.duration')" width="90" />
        <el-table-column :label="t('table.actions')" width="260">
          <template #default="scope">
            <el-button size="small" @click="openDetail('request', scope.row)">{{ t('httpLogs.request') }}</el-button>
            <el-button size="small" @click="openDetail('response', scope.row)">{{ t('httpLogs.response') }}</el-button>
            <el-button size="small" type="primary" @click="openDetail('pair', scope.row)">{{ t('httpLogs.pair') }}</el-button>
          </template>
        </el-table-column>
      </el-table>

      <UnifiedPagination v-model:page="page" v-model:pageSize="pageSize" :total="total" />
    </el-card>

    <el-dialog v-model="detailVisible" :title="detailTitle" width="980px" :close-on-click-modal="false" @closed="resetDetail">
      <div v-if="detailMode !== 'request'" style="margin-bottom: 10px">
        <el-switch v-model="decodeGzip" :disabled="!detailRow?.is_gzipped" @change="refetchResponseBody" />
        <span style="margin-left: 8px; opacity: 0.85">{{ t('httpLogs.decodeGzip') }}</span>
      </div>

      <el-alert
        v-if="responseBodyTruncated && responseBodyTruncatedReason"
        :title="t('httpLogs.truncated', { reason: responseBodyTruncatedReason })"
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom: 10px"
      />

      <template v-if="detailMode === 'request'">
        <el-tabs v-model="requestTab">
          <el-tab-pane name="h" :label="t('httpLogs.headers')">
            <pre class="mono">{{ requestHeader || t('common.loading') }}</pre>
          </el-tab-pane>
          <el-tab-pane name="b" :label="t('httpLogs.body')">
            <pre class="mono">{{ requestBody || t('common.loading') }}</pre>
          </el-tab-pane>
        </el-tabs>
      </template>

      <template v-else-if="detailMode === 'response'">
        <el-tabs v-model="responseTab">
          <el-tab-pane name="h" :label="t('httpLogs.headers')">
            <pre class="mono">{{ responseHeader || t('common.loading') }}</pre>
          </el-tab-pane>
          <el-tab-pane name="b" :label="t('httpLogs.body')">
            <pre class="mono">{{ responseBody || t('common.loading') }}</pre>
          </el-tab-pane>
        </el-tabs>
      </template>

      <template v-else>
        <el-tabs v-model="pairTab">
          <el-tab-pane name="req" :label="t('httpLogs.request')">
            <el-tabs v-model="requestTab">
              <el-tab-pane name="h" :label="t('httpLogs.headers')">
                <pre class="mono">{{ requestHeader || t('common.loading') }}</pre>
              </el-tab-pane>
              <el-tab-pane name="b" :label="t('httpLogs.body')">
                <pre class="mono">{{ requestBody || t('common.loading') }}</pre>
              </el-tab-pane>
            </el-tabs>
          </el-tab-pane>
          <el-tab-pane name="resp" :label="t('httpLogs.response')">
            <el-tabs v-model="responseTab">
              <el-tab-pane name="h" :label="t('httpLogs.headers')">
                <pre class="mono">{{ responseHeader || t('common.loading') }}</pre>
              </el-tab-pane>
              <el-tab-pane name="b" :label="t('httpLogs.body')">
                <pre class="mono">{{ responseBody || t('common.loading') }}</pre>
              </el-tab-pane>
            </el-tabs>
          </el-tab-pane>
        </el-tabs>
      </template>

      <template #footer>
        <el-button @click="detailVisible = false">{{ t('common.close') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

import { api } from "@/api/client";
import type { HTTPLog, HTTPLogPartResult, HTTPLogsPageResponse } from "@/api/types";
import UnifiedPagination from "@/components/UnifiedPagination.vue";
import { formatDateTime } from "@/utils/format";

const { t } = useI18n();

const loading = ref(false);
const rows = ref<HTTPLog[]>([]);
const total = ref(0);

const page = ref(1);
const pageSize = ref(20);

const q = ref("");

async function fetchLogs() {
  loading.value = true;
  try {
    const res = await api.get<HTTPLogsPageResponse>("/http-logs", {
      params: {
        page: page.value,
        page_size: pageSize.value,
        q: q.value.trim() || undefined,
      },
    });
    rows.value = res.data.items;
    total.value = res.data.total;
  } finally {
    loading.value = false;
  }
}

function search() {
  page.value = 1;
  fetchLogs().catch(() => undefined);
}

function reset() {
  q.value = "";
  page.value = 1;
  fetchLogs().catch(() => undefined);
}

watch([page, pageSize], () => {
  fetchLogs().catch(() => undefined);
});

onMounted(() => {
  fetchLogs().catch(() => undefined);
});

type DetailMode = "request" | "response" | "pair";
const detailVisible = ref(false);
const detailMode = ref<DetailMode>("request");
const detailRow = ref<HTTPLog | null>(null);

const detailTitle = computed(() => {
  const id = detailRow.value?.id ?? "-";
  if (detailMode.value === "request") return `${t("httpLogs.request")} #${id}`;
  if (detailMode.value === "response") return `${t("httpLogs.response")} #${id}`;
  return `${t("httpLogs.pair")} #${id}`;
});

const requestTab = ref("h");
const responseTab = ref("h");
const pairTab = ref("req");

const requestHeader = ref("");
const requestBody = ref("");
const responseHeader = ref("");
const responseBody = ref("");

const decodeGzip = ref(false);
const responseBodyTruncated = ref(false);
const responseBodyTruncatedReason = ref("");

async function fetchPart(id: number, part: string, decode?: boolean): Promise<HTTPLogPartResult> {
  const res = await api.get<HTTPLogPartResult>(`/http-logs/${id}/parts/${part}`, {
    params: {
      decode: decode ? "gzip" : undefined,
    },
  });
  return res.data;
}

async function refetchResponseBody() {
  const row = detailRow.value;
  if (!row) return;

  const result = await fetchPart(row.id, "response_body", decodeGzip.value);
  responseBody.value = result.data;
  responseBodyTruncated.value = Boolean(result.truncated);
  responseBodyTruncatedReason.value = result.truncated_reason || "";
}

async function openDetail(mode: DetailMode, row: HTTPLog) {
  detailMode.value = mode;
  detailRow.value = row;
  detailVisible.value = true;

  requestHeader.value = "";
  requestBody.value = "";
  responseHeader.value = "";
  responseBody.value = "";
  decodeGzip.value = false;
  responseBodyTruncated.value = false;
  responseBodyTruncatedReason.value = "";

  requestTab.value = "h";
  responseTab.value = "h";
  pairTab.value = "req";

  if (mode === "request") {
    const [h, b] = await Promise.all([
      fetchPart(row.id, "request_header"),
      fetchPart(row.id, "request_body"),
    ]);
    requestHeader.value = h.data;
    requestBody.value = b.data;
    return;
  }

  if (mode === "response") {
    const [h] = await Promise.all([fetchPart(row.id, "response_header")]);
    responseHeader.value = h.data;
    await refetchResponseBody();
    return;
  }

  const [rh, rb, sh] = await Promise.all([
    fetchPart(row.id, "request_header"),
    fetchPart(row.id, "request_body"),
    fetchPart(row.id, "response_header"),
  ]);
  requestHeader.value = rh.data;
  requestBody.value = rb.data;
  responseHeader.value = sh.data;
  await refetchResponseBody();
}

function resetDetail() {
  detailRow.value = null;
  requestHeader.value = "";
  requestBody.value = "";
  responseHeader.value = "";
  responseBody.value = "";
  decodeGzip.value = false;
  responseBodyTruncated.value = false;
  responseBodyTruncatedReason.value = "";
}
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.filters {
  display: flex;
  align-items: center;
  gap: 10px;
}
</style>
