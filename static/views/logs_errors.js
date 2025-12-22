import { apiGET } from "../app/api.js";

const { ref, inject, computed, onMounted, onDeactivated } = Vue;

export default {
  name: "ViewErrorLogs",
  setup() {
    const t = inject("t");
    const currentLang = inject("currentLang");
    const ElMessage = ElementPlus.ElMessage;

    const errorLogs = ref([]);
    const loading = ref(false);
    const detailVisible = ref(false);
    const selectedLog = ref(null);

    const locale = computed(() => (currentLang.value === "zh" ? "zh-CN" : "en-US"));

    const loadErrorLogs = async () => {
      loading.value = true;
      try {
        errorLogs.value = await apiGET("/error-logs");
      } catch (e) {
        ElMessage.error(e.message);
      } finally {
        loading.value = false;
      }
    };

    const refreshLogs = async () => {
      await loadErrorLogs();
      ElMessage.success(currentLang.value === "zh" ? "刷新成功" : "Refreshed");
    };

    const clearLogs = async () => {
      try {
        await fetch("/api/error-logs", { method: "DELETE" });
        await loadErrorLogs();
        ElMessage.success(currentLang.value === "zh" ? "清空成功" : "Cleared");
      } catch (e) {
        ElMessage.error(e.message);
      }
    };

    const showDetail = (row) => {
      selectedLog.value = row;
      detailVisible.value = true;
    };

    const closeDetail = () => {
      detailVisible.value = false;
      selectedLog.value = null;
    };

    const formatTime = (timestamp) => {
      if (!timestamp) return "-";
      const date = new Date(timestamp);
      return date.toLocaleString(locale.value, { hour12: false });
    };

    const formatJSON = (jsonStr) => {
      try {
        return JSON.stringify(JSON.parse(jsonStr), null, 2);
      } catch {
        return jsonStr;
      }
    };

    onMounted(() => {
      loadErrorLogs();
    });

    onDeactivated(() => {
      closeDetail();
    });

    return {
      t,
      currentLang,
      errorLogs,
      loading,
      detailVisible,
      selectedLog,
      loadErrorLogs,
      refreshLogs,
      clearLogs,
      showDetail,
      closeDetail,
      formatTime,
      formatJSON,
    };
  },
  template: `
    <div>
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom: 12px;gap:12px;">
        <div>
          <h2 style="margin: 0">{{ t.errorLogs }}</h2>
        </div>
        <div style="display:flex;gap:10px;align-items:center;flex-wrap:wrap;">
          <el-button @click="refreshLogs" icon="Refresh" :loading="loading">{{ t.refresh }}</el-button>
          <el-popconfirm :title="t.clearConfirm" @confirm="clearLogs">
            <template #reference>
              <el-button type="danger" icon="Delete">{{ t.clear }}</el-button>
            </template>
          </el-popconfirm>
        </div>
      </div>

      <el-card shadow="never">
        <template #header>
          <div style="display:flex;justify-content:space-between;align-items:center;flex-wrap:wrap;gap:10px;">
            <b>{{ t.recentErrors }} ({{ t.last100 }})</b>
            <el-tag v-if="errorLogs.length === 0" type="success">{{ t.noErrors }}</el-tag>
            <el-tag v-else type="warning">{{ errorLogs.length }} {{ t.errors }}</el-tag>
          </div>
        </template>

        <el-table
          :data="errorLogs"
          size="small"
          style="width: 100%"
          v-loading="loading"
          @row-click="showDetail"
        >
          <el-table-column prop="id" label="ID" width="60"></el-table-column>

          <el-table-column :label="t.time" width="180">
            <template #default="s">
              {{ formatTime(s.row.timestamp) }}
            </template>
          </el-table-column>

          <el-table-column :label="t.level" width="80">
            <template #default="s">
              <el-tag v-if="s.row.level === 'FATAL'" type="danger" size="small">FATAL</el-tag>
              <el-tag v-else-if="s.row.level === 'ERROR'" type="danger" size="small" effect="plain">ERROR</el-tag>
              <el-tag v-else type="warning" size="small">WARN</el-tag>
            </template>
          </el-table-column>

          <el-table-column prop="source" :label="t.source" width="150"></el-table-column>

          <el-table-column prop="message" :label="t.message" min-width="200">
            <template #default="s">
              <div style="max-width: 520px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
                {{ s.row.message }}
              </div>
            </template>
          </el-table-column>

          <el-table-column :label="t.detail" min-width="200">
            <template #default="s">
              <div v-if="s.row.detail" style="max-width: 520px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;" class="code">
                {{ s.row.detail }}
              </div>
              <span v-else style="color: #909399">-</span>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <el-dialog v-model="detailVisible" :title="t.errorDetail" width="820px" @close="closeDetail">
        <div v-if="selectedLog">
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="ID">{{ selectedLog.id }}</el-descriptions-item>
            <el-descriptions-item :label="t.time">{{ formatTime(selectedLog.timestamp) }}</el-descriptions-item>
            <el-descriptions-item :label="t.level">{{ selectedLog.level }}</el-descriptions-item>
            <el-descriptions-item :label="t.source">{{ selectedLog.source }}</el-descriptions-item>
            <el-descriptions-item :label="t.message">{{ selectedLog.message }}</el-descriptions-item>
            <el-descriptions-item v-if="selectedLog.detail" :label="t.detail">
              <div class="code">{{ selectedLog.detail }}</div>
            </el-descriptions-item>
          </el-descriptions>

          <div v-if="selectedLog.context" class="error-detail" style="margin-top: 15px">
            <h4 style="margin: 0 0 10px 0">{{ t.context }}:</h4>
            <pre class="code">{{ formatJSON(selectedLog.context) }}</pre>
          </div>

          <div v-if="selectedLog.stack" class="error-detail" style="margin-top: 15px">
            <h4 style="margin: 0 0 10px 0">{{ t.stackTrace }}:</h4>
            <div class="stack-trace">{{ selectedLog.stack }}</div>
          </div>
        </div>
      </el-dialog>
    </div>
  `,
};
