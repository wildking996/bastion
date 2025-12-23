import { apiDELETE, apiGET, apiPOST, apiPUT } from "../app/api.js";

const { ref, inject, onMounted, nextTick, onDeactivated } = Vue;

const PAGINATION_LAYOUT = "total, sizes, prev, pager, next, jumper";
const PAGE_SIZES = [10, 20, 50, 100];

export default {
  name: "ViewMappings",
  setup() {
    const t = inject("t");
    const ElMessage = ElementPlus.ElMessage;

    const loading = ref(false);
    const rows = ref([]);

    const page = ref(1);
    const pageSize = ref(10);
    const total = ref(0);

    const bastions = ref([]);

    const dialogVisible = ref(false);
    const dialogMode = ref("create");

    const portDefaults = { tcp: 1080, socks5: 1080, http: 8000 };
    const defaultForm = {
      id: "",
      type: "tcp",
      local_host: "127.0.0.1",
      local_port: portDefaults.tcp,
      remote_host: "10.0.0.1",
      remote_port: 22,
      chain: [],
      allow_cidrs: [],
      deny_cidrs: [],
      auto_start: false,
    };

    const form = ref({ ...defaultForm });
    const editingID = ref(null);

    // Per-mapping traffic chart
    const trafficDialogVisible = ref(false);
    const trafficDialogTitle = ref("");
    const trafficChartEl = ref(null);
    let trafficChart = null;
    let trafficTimer = null;
    let lastSample = null;
    const trafficSeries = ref({ times: [], upRate: [], downRate: [], conns: [] });

    const resetTraffic = () => {
      trafficSeries.value = { times: [], upRate: [], downRate: [], conns: [] };
      lastSample = null;
    };

    const stopTraffic = () => {
      if (trafficTimer) {
        clearInterval(trafficTimer);
        trafficTimer = null;
      }
      if (trafficChart) {
        try {
          trafficChart.dispose();
        } catch {
          // ignore
        }
        trafficChart = null;
      }
    };

    const formatRate = (v) => {
      const b = Number(v || 0);
      if (b >= 1024 * 1024) return (b / 1024 / 1024).toFixed(1) + " MB/s";
      if (b >= 1024) return (b / 1024).toFixed(1) + " KB/s";
      return Math.round(b) + " B/s";
    };

    const renderTraffic = () => {
      if (!trafficChartEl.value || !window.echarts) return;
      if (!trafficChart) trafficChart = window.echarts.init(trafficChartEl.value);

      trafficChart.setOption({
        tooltip: { trigger: "axis" },
        legend: { data: [t.value.upRate, t.value.downRate, t.value.connections] },
        grid: { left: 60, right: 20, top: 30, bottom: 30 },
        xAxis: {
          type: "category",
          boundaryGap: false,
          data: trafficSeries.value.times,
        },
        yAxis: [
          {
            type: "value",
            name: t.value.bytesPerSecond,
            axisLabel: { formatter: (val) => formatRate(val) },
          },
          { type: "value", name: t.value.connections },
        ],
        series: [
          {
            name: t.value.upRate,
            type: "line",
            showSymbol: false,
            smooth: true,
            data: trafficSeries.value.upRate,
          },
          {
            name: t.value.downRate,
            type: "line",
            showSymbol: false,
            smooth: true,
            data: trafficSeries.value.downRate,
          },
          {
            name: t.value.connections,
            type: "line",
            yAxisIndex: 1,
            showSymbol: false,
            smooth: true,
            data: trafficSeries.value.conns,
          },
        ],
      });
    };

    const sampleTraffic = async (mappingID) => {
      const stats = await apiGET("/stats");
      const s = stats[mappingID];
      if (!s) return null;
      return {
        up: Number(s.up_bytes || 0),
        down: Number(s.down_bytes || 0),
        conns: Number(s.connections || 0),
        ts: Date.now(),
      };
    };

    const startTraffic = (mappingID) => {
      stopTraffic();
      resetTraffic();

      const tick = async () => {
        try {
          const cur = await sampleTraffic(mappingID);
          if (!cur) return;

          const label = new Date(cur.ts).toLocaleTimeString(undefined, {
            hour12: false,
            hour: "2-digit",
            minute: "2-digit",
            second: "2-digit",
          });

          let upRate = 0;
          let downRate = 0;
          if (lastSample) {
            const dt = Math.max(1, (cur.ts - lastSample.ts) / 1000);
            upRate = (cur.up - lastSample.up) / dt;
            downRate = (cur.down - lastSample.down) / dt;
          }
          lastSample = cur;

          trafficSeries.value.times.push(label);
          trafficSeries.value.upRate.push(Math.max(0, upRate));
          trafficSeries.value.downRate.push(Math.max(0, downRate));
          trafficSeries.value.conns.push(cur.conns);

          const maxPoints = 60;
          if (trafficSeries.value.times.length > maxPoints) {
            trafficSeries.value.times.shift();
            trafficSeries.value.upRate.shift();
            trafficSeries.value.downRate.shift();
            trafficSeries.value.conns.shift();
          }

          renderTraffic();
        } catch {
          // ignore
        }
      };

      tick();
      trafficTimer = setInterval(tick, 2000);
    };

    const openTraffic = async (row) => {
      if (!row.running) {
        ElMessage.warning(t.value.mappingNotRunning);
        return;
      }
      trafficDialogTitle.value = `${t.value.trafficDashboard} - ${row.id}`;
      trafficDialogVisible.value = true;
      await nextTick();
      startTraffic(row.id);
    };

    const closeTraffic = () => {
      trafficDialogVisible.value = false;
      stopTraffic();
    };

    const loadBastions = async () => {
      try {
        bastions.value = await apiGET("/bastions");
      } catch {
        bastions.value = [];
      }
    };

    const loadPage = async () => {
      loading.value = true;
      try {
        const params = new URLSearchParams();
        params.set("page", String(page.value));
        params.set("page_size", String(pageSize.value));

        const resp = await fetch(`/api/mappings?${params.toString()}`);
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

    const openCreate = () => {
      dialogMode.value = "create";
      editingID.value = null;
      form.value = { ...defaultForm };
      dialogVisible.value = true;
    };

    const handleTypeChange = (value) => {
      if (dialogMode.value !== "edit") {
        form.value.local_port = portDefaults[value];
      }
      if (value === "tcp") {
        form.value.remote_host = "10.0.0.1";
        form.value.remote_port = 22;
      } else {
        form.value.remote_host = "0.0.0.0";
        form.value.remote_port = 0;
      }
    };

    const openEdit = (row) => {
      dialogMode.value = "edit";
      editingID.value = row.id;
      form.value = JSON.parse(JSON.stringify(row));
      form.value.id = row.id;
      dialogVisible.value = true;
    };

    const openCopyModify = (row) => {
      dialogMode.value = "create";
      editingID.value = null;
      form.value = JSON.parse(JSON.stringify(row));
      delete form.value.id;
      form.value.local_port = Number(row.local_port || 0) + 1;
      dialogVisible.value = true;
    };

    const closeDialog = () => {
      dialogVisible.value = false;
    };

    const submit = async () => {
      try {
        if (!form.value.local_port) {
          ElMessage.warning(t.value.localPort);
          return;
        }

        if (dialogMode.value === "edit" && editingID.value) {
          await apiPUT(`/mappings/${editingID.value}`, form.value);
          ElMessage.success(t.value.mappingSaved);
        } else {
          await apiPOST("/mappings", form.value);
          ElMessage.success(t.value.mappingCreated);
        }

        closeDialog();
        await loadPage();
      } catch (e) {
        ElMessage.error(e.message);
      }
    };

    const del = async (row) => {
      try {
        await apiDELETE(`/mappings/${row.id}`);
        ElMessage.success(t.value.deleted);
        if (rows.value.length === 1 && page.value > 1) page.value -= 1;
        await loadPage();
      } catch (e) {
        ElMessage.error(e.message);
      }
    };

    const startMapping = async (row) => {
      try {
        await apiPOST(`/mappings/${row.id}/start`, {});
        await loadPage();
      } catch (e) {
        ElMessage.error(e.message);
      }
    };

    const stopMapping = async (row) => {
      try {
        await apiPOST(`/mappings/${row.id}/stop`, {});
        await loadPage();
      } catch (e) {
        ElMessage.error(e.message);
      }
    };

    const toggleAutoStart = async (row) => {
      try {
        await apiPUT(`/mappings/${row.id}`, row);
        ElMessage.success(t.value.autoStartUpdated);
      } catch (e) {
        ElMessage.error(e.message);
        row.auto_start = !row.auto_start;
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

    onMounted(async () => {
      await loadBastions();
      await loadPage();
    });

    onDeactivated(() => {
      stopTraffic();
    });

    return {
      t,
      loading,
      rows,
      page,
      pageSize,
      total,
      bastions,
      dialogVisible,
      dialogMode,
      form,
      openCreate,
      openEdit,
      openCopyModify,
      closeDialog,
      submit,
      del,
      startMapping,
      stopMapping,
      toggleAutoStart,
      handleTypeChange,
      PAGINATION_LAYOUT,
      PAGE_SIZES,
      handleSizeChange,
      handleCurrentChange,
      trafficDialogVisible,
      trafficDialogTitle,
      trafficChartEl,
      openTraffic,
      closeTraffic,
    };
  },
  template: `
    <div>
      <el-row justify="space-between" align="middle" style="margin-bottom: 12px" :gutter="12">
        <el-col :span="12">
          <h2 style="margin: 0">{{ t.navMappings }}</h2>
        </el-col>
        <el-col :span="12" style="display:flex;justify-content:flex-end">
          <el-space wrap>
            <el-button type="primary" icon="Plus" @click="openCreate">{{ t.add }}</el-button>
          </el-space>
        </el-col>
      </el-row>

      <el-card shadow="never">
        <el-table :data="rows" v-loading="loading" size="default" style="width: 100%">
          <el-table-column prop="id" label="ID" min-width="220" />
          <el-table-column :label="t.type" width="90">
            <template #default="s">
              <el-tag>{{ s.row.type }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t.local" min-width="190">
            <template #default="s">
              <div class="code">{{ s.row.local_host }}:{{ s.row.local_port }}</div>
            </template>
          </el-table-column>
          <el-table-column :label="t.remote" min-width="190">
            <template #default="s">
              <div class="code">{{ s.row.remote_host }}:{{ s.row.remote_port }}</div>
            </template>
          </el-table-column>
          <el-table-column :label="t.chain" min-width="220">
            <template #default="s">
              <div class="muted">{{ (s.row.chain || []).join(' -> ') || '-' }}</div>
            </template>
          </el-table-column>
          <el-table-column :label="t.autoStart" width="120">
            <template #default="s">
              <el-switch v-model="s.row.auto_start" @change="toggleAutoStart(s.row)" />
            </template>
          </el-table-column>
          <el-table-column :label="t.status" width="110">
            <template #default="s">
              <el-tag v-if="s.row.running" type="success">{{ t.running }}</el-tag>
              <el-tag v-else type="info">{{ t.stopped }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t.operation" width="260" align="right">
            <template #default="s">
              <el-space>
                <el-tooltip :content="s.row.running ? t.stop : t.start" placement="top">
                  <template #reference>
                    <el-button
                      size="small"
                      type="primary"
                      :icon="s.row.running ? 'VideoPause' : 'VideoPlay'"
                      @click="s.row.running ? stopMapping(s.row) : startMapping(s.row)"
                      circle
                    />
                  </template>
                </el-tooltip>

                <el-tooltip :content="t.edit" placement="top">
                  <template #reference>
                    <el-button size="small" icon="Edit" @click="openEdit(s.row)" circle />
                  </template>
                </el-tooltip>

                <el-tooltip :content="t.copyModify" placement="top">
                  <template #reference>
                    <el-button size="small" icon="DocumentCopy" @click="openCopyModify(s.row)" circle />
                  </template>
                </el-tooltip>

                <el-tooltip :content="t.trafficDashboard" placement="top">
                  <template #reference>
                    <el-button size="small" icon="TrendCharts" @click="openTraffic(s.row)" circle />
                  </template>
                </el-tooltip>

                <el-popconfirm :title="t.deleteConfirm" @confirm="del(s.row)">
                  <template #reference>
                    <el-button size="small" type="danger" icon="Delete" circle />
                  </template>
                </el-popconfirm>
              </el-space>
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

      <el-dialog v-model="dialogVisible" :title="dialogMode === 'edit' ? t.editMapping : t.addMapping" width="760px" :close-on-click-modal="false">
        <el-form label-width="140px" size="default">
          <el-form-item :label="t.type">
            <el-radio-group v-model="form.type" @change="handleTypeChange" :disabled="dialogMode === 'edit'">
              <el-radio-button label="tcp">tcp</el-radio-button>
              <el-radio-button label="socks5">socks5</el-radio-button>
              <el-radio-button label="http">http</el-radio-button>
            </el-radio-group>
          </el-form-item>

          <el-form-item :label="t.local">
            <el-row :gutter="10" style="width: 100%">
              <el-col :span="14">
                <el-input v-model="form.local_host" :disabled="dialogMode === 'edit'" />
              </el-col>
              <el-col :span="10">
                <el-input-number v-model="form.local_port" :min="1" :max="65535" :disabled="dialogMode === 'edit'" style="width: 100%" />
              </el-col>
            </el-row>
          </el-form-item>

          <el-form-item :label="t.remote" v-if="form.type === 'tcp'">
            <el-row :gutter="10" style="width: 100%">
              <el-col :span="14">
                <el-input v-model="form.remote_host" :disabled="dialogMode === 'edit'" />
              </el-col>
              <el-col :span="10">
                <el-input-number v-model="form.remote_port" :min="1" :max="65535" :disabled="dialogMode === 'edit'" style="width: 100%" />
              </el-col>
            </el-row>
          </el-form-item>

          <el-form-item :label="t.chain">
            <el-select v-model="form.chain" multiple filterable clearable style="width: 100%">
              <el-option v-for="b in bastions" :key="b.name" :label="b.name" :value="b.name" />
            </el-select>
          </el-form-item>

          <el-form-item :label="t.ipAllowlist">
            <el-select v-model="form.allow_cidrs" multiple filterable allow-create default-first-option clearable style="width: 100%" :placeholder="t.cidrHint" />
          </el-form-item>

          <el-form-item :label="t.ipDenylist">
            <el-select v-model="form.deny_cidrs" multiple filterable allow-create default-first-option clearable style="width: 100%" :placeholder="t.cidrHint" />
          </el-form-item>

          <el-form-item :label="t.autoStart">
            <el-switch v-model="form.auto_start" />
          </el-form-item>
        </el-form>

        <template #footer>
          <el-button @click="closeDialog">{{ t.cancel }}</el-button>
          <el-button type="primary" @click="submit">{{ t.confirm }}</el-button>
        </template>
      </el-dialog>

      <el-dialog v-model="trafficDialogVisible" :title="trafficDialogTitle" width="920px" @close="closeTraffic">
        <div ref="trafficChartEl" style="height: 420px"></div>
        <template #footer>
          <el-button @click="closeTraffic">{{ t.close }}</el-button>
        </template>
      </el-dialog>
    </div>
  `,
};
