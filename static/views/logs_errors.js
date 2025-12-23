import { apiGET } from "../app/api.js";

const { ref, inject, computed, onMounted, onDeactivated } = Vue;

const PAGINATION_LAYOUT = "total, sizes, prev, pager, next, jumper";
const PAGE_SIZES = [10, 20, 50, 100];

export default {
  name: "ViewErrorLogs",
  setup() {
    const t = inject("t");
    const currentLang = inject("currentLang");
    const ElMessage = ElementPlus.ElMessage;

    const rows = ref([]);
    const loading = ref(false);

    const page = ref(1);
    const pageSize = ref(10);
    const total = ref(0);

    const detailVisible = ref(false);
    const selectedLog = ref(null);

    const locale = computed(() =>
      currentLang.value === "zh" ? "zh-CN" : "en-US"
    );

    const loadPage = async () => {
      loading.value = true;
      try {
        const params = new URLSearchParams();
        params.set("page", String(page.value));
        params.set("page_size", String(pageSize.value));

        const resp = await fetch(`/api/error-logs?${params.toString()}`);
        const data = await resp.json();
        if (!resp.ok) throw new Error(data.detail || resp.statusText);

        rows.value = data.data || [];
        total.value = Number(data.total || 0);
      } catch (e) {
        ElMessage.error(e.message);
      } finally {
        loading.value = false;
      }
    };

    const refreshLogs = async () => {
      await loadPage();
      ElMessage.success(t.value.refresh);
    };

    const clearLogs = async () => {
      try {
        await fetch("/api/error-logs", { method: "DELETE" });
        page.value = 1;
        await loadPage();
        ElMessage.success(t.value.logsCleared);
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

    const handleSizeChange = (s) => {
      pageSize.value = s;
      page.value = 1;
      loadPage();
    };

    const handleCurrentChange = (p) => {
      page.value = p;
      loadPage();
    };

    onMounted(() => {
      loadPage();
    });

    onDeactivated(() => {
      closeDetail();
    });

    return {
      t,
      currentLang,
      rows,
      loading,
      page,
      pageSize,
      total,
      detailVisible,
      selectedLog,
      refreshLogs,
      clearLogs,
      showDetail,
      closeDetail,
      formatTime,
      formatJSON,
      handleSizeChange,
      handleCurrentChange,
      PAGINATION_LAYOUT,
      PAGE_SIZES,
    };
  },
  template: `
    <div>
      <el-row justify="space-between" align="middle" style="margin-bottom: 12px" :gutter="12">
        <el-col :span="12">
          <h2 style="margin: 0">{{ t.navErrorLogs }}</h2>
        </el-col>
        <el-col :span="12" style="display:flex;justify-content:flex-end">
          <el-space wrap>
            <el-button icon="Refresh" @click="refreshLogs">{{ t.refresh }}</el-button>
            <el-popconfirm :title="t.clearConfirm" @confirm="clearLogs">
              <template #reference>
                <el-button type="danger" icon="Delete">{{ t.clear }}</el-button>
              </template>
            </el-popconfirm>
          </el-space>
        </el-col>
      </el-row>

      <el-card shadow="never">
        <el-table
          :data="rows"
          size="default"
          style="width: 100%"
          v-loading="loading"
          @row-click="showDetail"
        >
          <el-table-column prop="id" label="ID" width="80"></el-table-column>

          <el-table-column :label="t.time" width="190">
            <template #default="s">
              {{ formatTime(s.row.timestamp) }}
            </template>
          </el-table-column>

          <el-table-column :label="t.level" width="90">
            <template #default="s">
              <el-tag v-if="s.row.level === 'FATAL'" type="danger">FATAL</el-tag>
              <el-tag v-else-if="s.row.level === 'ERROR'" type="danger" effect="plain">ERROR</el-tag>
              <el-tag v-else type="warning">WARN</el-tag>
            </template>
          </el-table-column>

          <el-table-column prop="source" :label="t.source" width="220"></el-table-column>

          <el-table-column prop="message" :label="t.message" min-width="260">
            <template #default="s">
              <div style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width: 680px;">{{ s.row.message }}</div>
            </template>
          </el-table-column>

          <el-table-column :label="t.detail" min-width="240">
            <template #default="s">
              <div v-if="s.row.detail" class="code" style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width: 680px;">{{ s.row.detail }}</div>
              <span v-else class="muted">-</span>
            </template>
          </el-table-column>
        </el-table>

        <div style="display:flex;justify-content:flex-end;margin-top: 12px;">
          <el-pagination
            background
            :layout="PAGINATION_LAYOUT"
            :total="total"
            :page-size="pageSize"
            :current-page="page"
            :page-sizes="PAGE_SIZES"
            @size-change="handleSizeChange"
            @current-change="handleCurrentChange"
          />
        </div>
      </el-card>

      <el-dialog v-model="detailVisible" :title="t.errorDetail" width="900px" @close="closeDetail">
        <div v-if="selectedLog">
          <el-descriptions :column="1" border size="default">
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
