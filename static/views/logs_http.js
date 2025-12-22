const {
  ref,
  inject,
  computed,
  onMounted,
  onActivated,
  onDeactivated,
  onUnmounted,
} = Vue;

export default {
  name: "ViewHTTPLogs",
  setup() {
    const t = inject("t");
    const currentLang = inject("currentLang");
    const ElMessage = ElementPlus.ElMessage;

    const logs = ref([]);
    const loading = ref(false);

    const requestDialogVisible = ref(false);
    const responseDialogVisible = ref(false);
    const pairDialogVisible = ref(false);
    const currentLog = ref({});
    const decodingGzip = ref(false);

    const autoRefresh = ref(false);
    const refreshInterval = ref(60);
    let timer = null;
    let abortController = null;

    const currentPage = ref(1);
    const pageSize = ref(10);
    const total = ref(0);

    const filterQ = ref("");
    const filterRegex = ref(false);
    const filterMethod = ref("");
    const filterHost = ref("");
    const filterUrl = ref("");
    const filterBastion = ref("");
    const filterLocalPort = ref(null);
    const filterStatus = ref(null);
    const filterTimeRange = ref([]);

    const methodOptions = [
      "GET",
      "POST",
      "PUT",
      "DELETE",
      "PATCH",
      "HEAD",
      "OPTIONS",
      "CONNECT",
    ];

    const locale = computed(() =>
      currentLang.value === "zh" ? "zh-CN" : "en-US"
    );

    const stopAutoRefresh = () => {
      if (!timer) return;
      clearInterval(timer);
      timer = null;
    };

    const startAutoRefresh = () => {
      stopAutoRefresh();
      timer = setInterval(loadLogs, refreshInterval.value * 1000);
    };

    const abortInFlight = () => {
      if (!abortController) return;
      try {
        abortController.abort();
      } catch {
        // ignore
      }
      abortController = null;
    };

    const makeParams = () => {
      const params = new URLSearchParams();
      params.set("page", String(currentPage.value));
      params.set("page_size", String(pageSize.value));

      const q = (filterQ.value || "").trim();
      if (q) {
        params.set("q", q);
        if (filterRegex.value) params.set("regex", "true");
      }

      const method = (filterMethod.value || "").trim();
      if (method) params.set("method", method);

      const host = (filterHost.value || "").trim();
      if (host) params.set("host", host);

      const url = (filterUrl.value || "").trim();
      if (url) params.set("url", url);

      const bastion = (filterBastion.value || "").trim();
      if (bastion) params.set("bastion", bastion);

      const localPort = Number(filterLocalPort.value);
      if (Number.isFinite(localPort) && localPort > 0) {
        params.set("local_port", String(localPort));
      }

      const statusCode = Number(filterStatus.value);
      if (Number.isFinite(statusCode) && statusCode >= 0) {
        params.set("status", String(statusCode));
      }

      if (filterTimeRange.value && filterTimeRange.value.length === 2) {
        const since = filterTimeRange.value[0];
        const until = filterTimeRange.value[1];
        if (since) params.set("since", new Date(since).toISOString());
        if (until) params.set("until", new Date(until).toISOString());
      }

      return params;
    };

    const loadLogs = async () => {
      loading.value = true;
      abortInFlight();
      abortController = new AbortController();

      try {
        const params = makeParams();
        const resp = await fetch(`/api/http-logs?${params.toString()}`, {
          signal: abortController.signal,
        });
        const data = await resp.json();
        if (!resp.ok) {
          throw new Error(data.detail || resp.statusText);
        }
        logs.value = data.data || [];
        total.value = data.total || 0;
      } catch (e) {
        if (e && e.name === "AbortError") return;
        ElMessage.error(`${t.value.loadLogsFailed}: ${e.message}`);
      } finally {
        loading.value = false;
      }
    };

    const applyFilters = async () => {
      currentPage.value = 1;
      await loadLogs();
    };

    const resetFilters = async () => {
      filterQ.value = "";
      filterRegex.value = false;
      filterMethod.value = "";
      filterHost.value = "";
      filterUrl.value = "";
      filterBastion.value = "";
      filterLocalPort.value = null;
      filterStatus.value = null;
      filterTimeRange.value = [];
      currentPage.value = 1;
      await loadLogs();
    };

    const clearLogs = async () => {
      try {
        await fetch("/api/http-logs", { method: "DELETE" });
        ElMessage.success(t.value.logsCleared);
        currentPage.value = 1;
        await loadLogs();
      } catch (e) {
        ElMessage.error(`${t.value.clearFailed}: ${e.message}`);
      }
    };

    const closeDetails = () => {
      requestDialogVisible.value = false;
      responseDialogVisible.value = false;
      pairDialogVisible.value = false;
      decodingGzip.value = false;
    };

    const showRequest = (log) => {
      currentLog.value = log;
      requestDialogVisible.value = true;
    };

    const showResponse = (log) => {
      currentLog.value = log;
      responseDialogVisible.value = true;
    };

    const showPair = (log) => {
      currentLog.value = log;
      pairDialogVisible.value = true;
    };

    const copyContent = (content) => {
      if (!content) {
        ElMessage.warning(t.value.noContentToCopy);
        return;
      }
      navigator.clipboard.writeText(content);
      ElMessage.success(t.value.copiedToClipboard);
    };

    const getDisplayResponse = (log) => {
      if (log.is_gzipped && log.response_decoded) return log.response_decoded;
      return log.response;
    };

    const copyBoth = () => {
      const response = getDisplayResponse(currentLog.value);
      const reqLabel = currentLang.value === "zh" ? "HTTP 请求" : "HTTP Request";
      const respLabel =
        currentLang.value === "zh" ? "HTTP 响应" : "HTTP Response";
      const none = currentLang.value === "zh" ? "无" : "None";
      const content = `=== ${reqLabel} ===\n${currentLog.value.request || none}\n\n=== ${respLabel} ===\n${response || none}`;
      navigator.clipboard.writeText(content);
      ElMessage.success(t.value.copiedRequestAndResponse);
    };

    const decodeGzipResponseBody = async (log) => {
      if (!log || !log.id) return;
      decodingGzip.value = true;
      try {
        const resp = await fetch(
          `/api/http-logs/${log.id}?part=response_body&decode=gzip`
        );
        const data = await resp.json();
        if (!resp.ok) {
          throw new Error(data.detail || resp.statusText);
        }

        log.response_decoded = data.data || "";

        if (data.truncated_reason) {
          ElMessage.warning(
            `${t.value.decodeGzipTruncated}: ${data.truncated_reason}`
          );
        } else {
          ElMessage.success(t.value.decodeGzipSuccess);
        }
      } catch (e) {
        ElMessage.error(`${t.value.decodeGzipFailed}: ${e.message}`);
      } finally {
        decodingGzip.value = false;
      }
    };

    const formatTime = (timestamp) => {
      if (!timestamp) return "-";
      const d = new Date(timestamp);
      return d.toLocaleString(locale.value, {
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
        hour12: false,
      });
    };

    const formatBytes = (bytes) => {
      if (!bytes || bytes === 0) return "0 B";
      const k = 1024;
      const sizes = ["B", "KB", "MB", "GB"];
      const i = Math.floor(Math.log(bytes) / Math.log(k));
      return (bytes / Math.pow(k, i)).toFixed(1) + " " + sizes[i];
    };

    const getMethodType = (method) => {
      const types = {
        GET: "success",
        POST: "primary",
        PUT: "warning",
        DELETE: "danger",
        PATCH: "warning",
        HEAD: "info",
        OPTIONS: "info",
      };
      return types[method] || "";
    };

    const getStatusType = (statusCode) => {
      const code = Number(statusCode);
      if (!Number.isFinite(code) || code <= 0) return "info";
      if (code >= 200 && code < 300) return "success";
      if (code >= 300 && code < 400) return "info";
      if (code >= 400 && code < 500) return "warning";
      return "danger";
    };

    const getFullUrl = (log) => {
      if (!log.host && !log.url) return "-";

      let protocol = "http";
      if (log.protocol && log.protocol.toLowerCase().includes("https")) {
        protocol = "https";
      }

      const host = log.host || "";
      const url = log.url || "";

      if (url.startsWith("http://") || url.startsWith("https://")) return url;
      if (host && url) return `${protocol}://${host}${url}`;
      if (host) return `${protocol}://${host}`;
      return url;
    };

    const handlePageChange = (page) => {
      currentPage.value = page;
      loadLogs();
    };

    const handleSizeChange = (size) => {
      pageSize.value = size;
      currentPage.value = 1;
      loadLogs();
    };

    const toggleAutoRefresh = (val) => {
      if (val) startAutoRefresh();
      else stopAutoRefresh();
    };

    const updateRefreshInterval = () => {
      if (!autoRefresh.value) return;
      stopAutoRefresh();
      startAutoRefresh();
    };

    onMounted(() => {
      loadLogs();
    });

    onActivated(() => {
      if (autoRefresh.value) startAutoRefresh();
    });

    onDeactivated(() => {
      stopAutoRefresh();
      abortInFlight();
      closeDetails();
    });

    onUnmounted(() => {
      stopAutoRefresh();
      abortInFlight();
    });

    return {
      t,
      currentLang,
      logs,
      loading,
      requestDialogVisible,
      responseDialogVisible,
      pairDialogVisible,
      currentLog,
      decodingGzip,
      autoRefresh,
      refreshInterval,
      currentPage,
      pageSize,
      total,
      loadLogs,
      clearLogs,
      closeDetails,
      showRequest,
      showResponse,
      showPair,
      copyContent,
      copyBoth,
      decodeGzipResponseBody,
      formatTime,
      formatBytes,
      getMethodType,
      getStatusType,
      getDisplayResponse,
      getFullUrl,
      handlePageChange,
      handleSizeChange,
      toggleAutoRefresh,
      updateRefreshInterval,
      filterQ,
      filterRegex,
      filterMethod,
      filterHost,
      filterUrl,
      filterBastion,
      filterLocalPort,
      filterStatus,
      filterTimeRange,
      methodOptions,
      applyFilters,
      resetFilters,
    };
  },
  template: `
    <div>
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom: 12px;gap:12px;">
        <div>
          <h2 style="margin: 0">{{ t.httpTrafficLogs }}</h2>
          <p style="margin: 5px 0 0; color: #909399; font-size: 13px">{{ t.logsDescription }}</p>
        </div>
        <div style="display:flex;gap:10px;align-items:center;flex-wrap:wrap;">
          <el-button @click="loadLogs" icon="Refresh" :loading="loading">{{ t.refresh }}</el-button>
          <el-popconfirm :title="t.clearAllLogs" @confirm="clearLogs">
            <template #reference>
              <el-button type="danger" icon="Delete">{{ t.clear }}</el-button>
            </template>
          </el-popconfirm>
        </div>
      </div>

      <el-card shadow="never">
        <template #header>
          <div style="display:flex;justify-content:space-between;align-items:center;flex-wrap:wrap;gap:10px;">
            <b>{{ t.logsList }}</b>
            <div style="display:flex;gap:12px;align-items:center;flex-wrap:wrap;">
              <span class="muted">{{ t.total }} {{ total }} {{ t.items }}</span>
              <el-switch v-model="autoRefresh" @change="toggleAutoRefresh" :active-text="t.autoRefresh"></el-switch>
              <el-input-number v-model="refreshInterval" :min="3" :max="3600" @change="updateRefreshInterval"></el-input-number>
              <span class="muted">{{ t.seconds }}</span>
            </div>
          </div>
        </template>

        <div style="margin-bottom: 12px;">
          <el-row :gutter="12">
            <el-col :span="8">
              <el-input v-model="filterQ" :placeholder="t.searchKeyword" clearable @keyup.enter="applyFilters" />
            </el-col>
            <el-col :span="4">
              <el-checkbox v-model="filterRegex">{{ t.regex }}</el-checkbox>
            </el-col>
            <el-col :span="4">
              <el-select v-model="filterMethod" clearable :placeholder="t.method" style="width: 100%">
                <el-option v-for="m in methodOptions" :key="m" :label="m" :value="m"></el-option>
              </el-select>
            </el-col>
            <el-col :span="4">
              <el-input v-model="filterHost" :placeholder="t.host" clearable />
            </el-col>
            <el-col :span="4">
              <el-input v-model="filterUrl" :placeholder="t.url" clearable />
            </el-col>
          </el-row>
          <el-row :gutter="12" style="margin-top: 10px;">
            <el-col :span="6">
              <el-input v-model="filterBastion" :placeholder="t.bastion" clearable />
            </el-col>
            <el-col :span="6">
              <el-input-number v-model="filterLocalPort" :min="1" :max="65535" style="width: 100%" :placeholder="t.localPort" />
            </el-col>
            <el-col :span="6">
              <el-input-number v-model="filterStatus" :min="0" :max="599" style="width: 100%" :placeholder="t.httpStatusCode" />
            </el-col>
            <el-col :span="6" style="display:flex;gap:10px;justify-content:flex-end;">
              <el-button type="primary" @click="applyFilters">{{ t.search }}</el-button>
              <el-button @click="resetFilters">{{ t.reset }}</el-button>
            </el-col>
          </el-row>

          <el-row :gutter="12" style="margin-top: 10px;">
            <el-col :span="24">
              <el-date-picker
                v-model="filterTimeRange"
                type="datetimerange"
                range-separator="-"
                :start-placeholder="t.since"
                :end-placeholder="t.until"
                style="width: 100%"
              />
            </el-col>
          </el-row>
        </div>

        <el-table :data="logs" size="small" style="width: 100%" v-loading="loading">
          <el-table-column prop="id" label="ID" width="80"></el-table-column>
          <el-table-column :label="t.time" width="170">
            <template #default="s">
              <div class="time-cell">{{ formatTime(s.row.timestamp) }}</div>
            </template>
          </el-table-column>
          <el-table-column :label="t.connection" width="120">
            <template #default="s">
              <div class="code">{{ s.row.local_port }}</div>
            </template>
          </el-table-column>
          <el-table-column :label="t.method" width="90">
            <template #default="s">
              <el-tag :type="getMethodType(s.row.method)" size="small">{{ s.row.method }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t.url" min-width="240">
            <template #default="s">
              <div style="display:flex;flex-direction:column;gap:2px;">
                <div class="code">{{ getFullUrl(s.row) }}</div>
                <div class="muted" style="font-size: 12px;">{{ (s.row.bastion_chain || []).join(' -> ') || '-' }}</div>
              </div>
            </template>
          </el-table-column>
          <el-table-column :label="t.status" width="110">
            <template #default="s">
              <el-tag :type="getStatusType(s.row.status_code)" size="small">{{ s.row.status_code || '-' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t.size" width="160">
            <template #default="s">
              <div class="size-cell">{{ formatBytes(s.row.req_size || 0) }} / {{ formatBytes(s.row.resp_size || 0) }}</div>
            </template>
          </el-table-column>
          <el-table-column :label="t.duration" width="110">
            <template #default="s">
              <div class="code">{{ (s.row.duration_ms || 0) + 'ms' }}</div>
            </template>
          </el-table-column>
          <el-table-column :label="t.operation" width="200">
            <template #default="s">
              <div style="display:flex;gap:6px;flex-wrap:wrap;">
                <el-button size="small" @click="showRequest(s.row)">{{ t.request }}</el-button>
                <el-button size="small" @click="showResponse(s.row)">{{ t.response }}</el-button>
                <el-button size="small" type="primary" @click="showPair(s.row)">{{ t.pair }}</el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>

        <div style="display:flex;justify-content:flex-end;margin-top: 12px;">
          <el-pagination
            background
            layout="sizes, prev, pager, next"
            :total="total"
            :page-size="pageSize"
            :current-page="currentPage"
            @current-change="handlePageChange"
            @size-change="handleSizeChange"
          ></el-pagination>
        </div>
      </el-card>

      <el-dialog v-model="requestDialogVisible" :title="t.httpRequestDetails" width="900px" @close="closeDetails">
        <div class="log-content">{{ currentLog.request || t.noRequest }}</div>
        <template #footer>
          <el-button @click="requestDialogVisible = false">{{ t.close }}</el-button>
          <el-button type="success" @click="copyContent(currentLog.request)">{{ t.copyRequest }}</el-button>
        </template>
      </el-dialog>

      <el-dialog v-model="responseDialogVisible" :title="t.httpResponseDetails" width="900px" @close="closeDetails">
        <div class="log-content">{{ getDisplayResponse(currentLog) || t.noResponse }}</div>
        <template #footer>
          <el-button @click="responseDialogVisible = false">{{ t.close }}</el-button>
          <el-button
            v-if="currentLog.is_gzipped && !currentLog.response_decoded"
            type="warning"
            :loading="decodingGzip"
            @click="decodeGzipResponseBody(currentLog)"
          >
            {{ t.decodeGzipBody }}
          </el-button>
          <el-button type="success" @click="copyContent(getDisplayResponse(currentLog))">{{ t.copyResponse }}</el-button>
        </template>
      </el-dialog>

      <el-dialog v-model="pairDialogVisible" :title="t.httpRequestResponsePair" width="1100px" @close="closeDetails">
        <el-row :gutter="12">
          <el-col :span="12">
            <h4 style="margin: 0 0 8px 0">{{ t.request }}</h4>
            <div class="log-content">{{ currentLog.request || t.noRequest }}</div>
          </el-col>
          <el-col :span="12">
            <h4 style="margin: 0 0 8px 0">{{ t.response }}</h4>
            <div class="log-content">{{ getDisplayResponse(currentLog) || t.noResponse }}</div>
          </el-col>
        </el-row>
        <template #footer>
          <el-button @click="pairDialogVisible = false">{{ t.close }}</el-button>
          <el-button type="success" @click="copyContent(currentLog.request)">{{ t.copyRequest }}</el-button>
          <el-button
            v-if="currentLog.is_gzipped && !currentLog.response_decoded"
            type="warning"
            :loading="decodingGzip"
            @click="decodeGzipResponseBody(currentLog)"
          >
            {{ t.decodeGzipBody }}
          </el-button>
          <el-button type="primary" @click="copyContent(getDisplayResponse(currentLog))">{{ t.copyResponse }}</el-button>
          <el-button type="warning" @click="copyBoth">{{ t.copyBoth }}</el-button>
        </template>
      </el-dialog>
    </div>
  `,
};
