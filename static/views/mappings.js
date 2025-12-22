import { apiDELETE, apiGET, apiPOST, apiPUT } from "../app/api.js";

const { ref, inject, onActivated, onDeactivated, onMounted, nextTick } = Vue;

export default {
  name: "ViewMappings",
  setup() {
    const t = inject("t");
    const currentLang = inject("currentLang");
    const ElMessage = ElementPlus.ElMessage;

    const bastions = ref([]);
    const mappings = ref([]);
    const trafficStats = ref({});
    const loading = ref({ maps: false });

    const portDefaults = { tcp: 1080, socks5: 1080, http: 8000 };
    const defaultMapping = {
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
    const mappingForm = ref({ ...defaultMapping });
    const isEditingMapping = ref(false);

    const loadBastions = async () => {
      bastions.value = await apiGET("/bastions");
    };

    const loadMappings = async () => {
      loading.value.maps = true;
      try {
        mappings.value = await apiGET("/mappings");
        if (mappings.value.length > 0) await loadStats();
      } finally {
        loading.value.maps = false;
      }
    };

    const loadStats = async () => {
      if (!mappings.value || mappings.value.length === 0) return;
      const hasRunning = mappings.value.some((m) => m.running);
      if (!hasRunning) return;
      try {
        trafficStats.value = await apiGET("/stats");
      } catch {
        // silent
      }
    };

    const sumStats = (stats) => {
      let upBytes = 0;
      let downBytes = 0;
      let connections = 0;
      if (!stats) return { upBytes, downBytes, connections };
      for (const s of Object.values(stats)) {
        if (!s) continue;
        upBytes += Number(s.up_bytes || 0);
        downBytes += Number(s.down_bytes || 0);
        connections += Number(s.connections || 0);
      }
      return { upBytes, downBytes, connections };
    };

    const formatBytes = (b) => {
      if (!b || b <= 0) return "0";
      const i = Math.floor(Math.log(b) / Math.log(1024));
      return (b / Math.pow(1024, i)).toFixed(1) + ["B", "K", "M", "G"][i];
    };

    // Traffic chart
    const trafficChartEl = ref(null);
    const trafficChartEnabled = ref(false);
    const trafficChartIntervalSeconds = ref(2);
    const trafficChartMaxPoints = 60;
    let trafficChart = null;
    let trafficChartTimer = null;
    let lastTrafficSample = null;
    const trafficSeries = ref({
      times: [],
      upRate: [],
      downRate: [],
      conns: [],
    });

    const renderTrafficChart = () => {
      if (!trafficChartEl.value) return;
      if (!window.echarts) return;
      if (!trafficChart) {
        trafficChart = window.echarts.init(trafficChartEl.value);
      }

      trafficChart.setOption({
        tooltip: { trigger: "axis" },
        legend: {
          textStyle: { color: "#e7e9ea" },
          data: [t.value.upRate, t.value.downRate, t.value.connections],
        },
        grid: { left: 60, right: 60, top: 40, bottom: 30 },
        xAxis: {
          type: "category",
          boundaryGap: false,
          data: trafficSeries.value.times,
          axisLabel: { color: "#909399" },
        },
        yAxis: [
          {
            type: "value",
            name: t.value.bytesPerSecond,
            axisLabel: {
              color: "#909399",
              formatter: (v) => formatBytes(v),
            },
            splitLine: { lineStyle: { color: "#1a1d23" } },
          },
          {
            type: "value",
            name: t.value.connections,
            axisLabel: { color: "#909399" },
            splitLine: { show: false },
          },
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

    const pushTrafficPoint = (timeLabel, upRate, downRate, connections) => {
      const series = trafficSeries.value;
      series.times.push(timeLabel);
      series.upRate.push(upRate);
      series.downRate.push(downRate);
      series.conns.push(connections);

      const overflow = series.times.length - trafficChartMaxPoints;
      if (overflow > 0) {
        series.times.splice(0, overflow);
        series.upRate.splice(0, overflow);
        series.downRate.splice(0, overflow);
        series.conns.splice(0, overflow);
      }
    };

    const refreshTrafficChart = async () => {
      try {
        const stats = await apiGET("/stats");
        const totals = sumStats(stats);

        const now = Date.now();
        let upRate = 0;
        let downRate = 0;
        if (lastTrafficSample) {
          const deltaSeconds = (now - lastTrafficSample.ts) / 1000;
          if (deltaSeconds > 0) {
            upRate = (totals.upBytes - lastTrafficSample.upBytes) / deltaSeconds;
            downRate =
              (totals.downBytes - lastTrafficSample.downBytes) / deltaSeconds;
          }
        }
        lastTrafficSample = {
          ts: now,
          upBytes: totals.upBytes,
          downBytes: totals.downBytes,
        };

        const label = new Date(now).toLocaleTimeString([], {
          hour12: false,
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit",
        });

        pushTrafficPoint(
          label,
          Math.max(0, upRate),
          Math.max(0, downRate),
          totals.connections
        );
        renderTrafficChart();
      } catch {
        // keep chart stable
      }
    };

    const stopTrafficChart = () => {
      if (trafficChartTimer) {
        clearInterval(trafficChartTimer);
        trafficChartTimer = null;
      }
    };

    const startTrafficChart = async () => {
      stopTrafficChart();
      await nextTick();
      await refreshTrafficChart();
      trafficChartTimer = setInterval(
        refreshTrafficChart,
        trafficChartIntervalSeconds.value * 1000
      );
    };

    const toggleTrafficChart = (enabled) => {
      if (enabled) {
        startTrafficChart();
      } else {
        stopTrafficChart();
      }
    };

    const updateTrafficChartInterval = () => {
      if (trafficChartEnabled.value) {
        startTrafficChart();
      }
    };

    // Mappings auto refresh
    const autoRefreshMappings = ref(false);
    const refreshInterval = ref(5000);
    let mappingRefreshInterval = null;
    let trafficRefreshInterval = null;

    const startMappingAutoRefresh = () => {
      stopMappingAutoRefresh();
      mappingRefreshInterval = setInterval(async () => {
        await loadMappings();
      }, refreshInterval.value);
      trafficRefreshInterval = setInterval(async () => {
        await loadStats();
      }, 2000);
    };

    const stopMappingAutoRefresh = () => {
      if (mappingRefreshInterval) {
        clearInterval(mappingRefreshInterval);
        mappingRefreshInterval = null;
      }
      if (trafficRefreshInterval) {
        clearInterval(trafficRefreshInterval);
        trafficRefreshInterval = null;
      }
    };

    const toggleMappingAutoRefresh = (enabled) => {
      if (enabled) startMappingAutoRefresh();
      else stopMappingAutoRefresh();
    };

    // Mapping CRUD
    const resetMappingForm = () => {
      mappingForm.value = { ...defaultMapping };
      isEditingMapping.value = false;
    };

    const handleMappingTypeChange = (value) => {
      if (!isEditingMapping.value) {
        mappingForm.value.local_port = portDefaults[value];
      }
      if (value === "tcp") {
        mappingForm.value.remote_host = "10.0.0.1";
        mappingForm.value.remote_port = 22;
      }
      if (value === "socks5" || value === "http") {
        mappingForm.value.remote_host = "0.0.0.0";
        mappingForm.value.remote_port = 0;
      }
    };

    const addMapping = async () => {
      try {
        if (isEditingMapping.value) {
          await apiPUT(`/mappings/${mappingForm.value.id}`, mappingForm.value);
          ElMessage.success(t.value.mappingSaved);
        } else {
          await apiPOST("/mappings", mappingForm.value);
          ElMessage.success(t.value.mappingCreated);
        }
        resetMappingForm();
        await loadMappings();
      } catch (e) {
        ElMessage.error(e.message);
      }
    };

    const editMapping = (row) => {
      mappingForm.value = JSON.parse(JSON.stringify(row));
      isEditingMapping.value = true;
    };

    const copyMapping = (row) => {
      mappingForm.value = JSON.parse(JSON.stringify(row));
      delete mappingForm.value.id;
      mappingForm.value.local_port += 1;
      isEditingMapping.value = false;
      ElMessage.info(t.value.configCopiedPortIncremented);
    };

    const delMapping = async (id) => {
      await apiDELETE(`/mappings/${id}`);
      loadMappings();
    };

    const startMapping = async (id) => {
      try {
        await apiPOST(`/mappings/${id}/start`, {});
        loadMappings();
      } catch (e) {
        ElMessage.error(e.message);
      }
    };

    const stopMapping = async (id) => {
      await apiPOST(`/mappings/${id}/stop`, {});
      loadMappings();
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

    const resizeTrafficChart = () => {
      if (trafficChart) trafficChart.resize();
    };

    onMounted(async () => {
      await loadBastions();
      await loadMappings();
      window.addEventListener("resize", resizeTrafficChart);
    });

    onActivated(() => {
      if (autoRefreshMappings.value) startMappingAutoRefresh();
      if (trafficChartEnabled.value) startTrafficChart();
    });

    onDeactivated(() => {
      stopMappingAutoRefresh();
      stopTrafficChart();
    });

    return {
      t,
      currentLang,
      bastions,
      mappings,
      trafficStats,
      loading,
      mappingForm,
      isEditingMapping,
      handleMappingTypeChange,
      addMapping,
      resetMappingForm,
      editMapping,
      copyMapping,
      delMapping,
      startMapping,
      stopMapping,
      toggleAutoStart,
      autoRefreshMappings,
      refreshInterval,
      toggleMappingAutoRefresh,
      trafficChartEl,
      trafficChartEnabled,
      trafficChartIntervalSeconds,
      toggleTrafficChart,
      updateTrafficChartInterval,
      formatBytes,
    };
  },
  template: `
    <div>
      <el-row :gutter="24">
        <el-col :span="6">
          <el-card shadow="never" :class="{ 'editing-card': isEditingMapping }">
            <template #header>
              <b>{{ isEditingMapping ? t.editMapping : t.addMapping }}</b>
              <el-button
                v-if="isEditingMapping"
                size="small"
                style="float: right"
                @click="resetMappingForm"
                >{{ t.cancel }}</el-button
              >
            </template>
            <el-form :model="mappingForm" label-width="60px" size="small">
              <el-form-item :label="t.type">
                <el-radio-group
                  v-model="mappingForm.type"
                  @change="handleMappingTypeChange"
                  :disabled="isEditingMapping"
                >
                  <el-radio-button label="tcp">TCP</el-radio-button>
                  <el-radio-button label="socks5">SOCKS5</el-radio-button>
                  <el-radio-button label="http">HTTP</el-radio-button>
                </el-radio-group>
              </el-form-item>

              <el-form-item :label="t.local">
                <el-input-number
                  v-model="mappingForm.local_port"
                  :min="1"
                  :max="65535"
                  style="width: 100%"
                  :disabled="isEditingMapping"
                />
                <div
                  v-if="isEditingMapping"
                  style="font-size: 10px; color: #909399; line-height: 1"
                >
                  {{ t.editLocalPortNote }}
                </div>
              </el-form-item>

              <template v-if="mappingForm.type === 'tcp'">
                <el-form-item :label="t.remote">
                  <el-input
                    v-model="mappingForm.remote_host"
                    placeholder="10.x.x.x"
                    :disabled="isEditingMapping"
                  />
                </el-form-item>
                <el-form-item :label="t.port">
                  <el-input-number
                    v-model="mappingForm.remote_port"
                    :min="1"
                    :max="65535"
                    style="width: 100%"
                    :disabled="isEditingMapping"
                  />
                </el-form-item>
              </template>
              <template v-else>
                <el-alert
                  :title="t.dynamicTarget"
                  type="info"
                  :closable="false"
                  style="margin-bottom: 18px"
                />
              </template>

              <el-form-item :label="t.chain">
                <el-select
                  v-model="mappingForm.chain"
                  multiple
                  filterable
                  :placeholder="t.selectOrder"
                  style="width: 100%"
                >
                  <el-option
                    v-for="b in bastions"
                    :key="b.name"
                    :label="b.name"
                    :value="b.name"
                  />
                </el-select>
              </el-form-item>

              <el-form-item :label="t.ipAllowlist">
                <el-select
                  v-model="mappingForm.allow_cidrs"
                  multiple
                  filterable
                  allow-create
                  default-first-option
                  :placeholder="t.cidrHint"
                  style="width: 100%"
                />
              </el-form-item>

              <el-form-item :label="t.ipDenylist">
                <el-select
                  v-model="mappingForm.deny_cidrs"
                  multiple
                  filterable
                  allow-create
                  default-first-option
                  :placeholder="t.cidrHint"
                  style="width: 100%"
                />
              </el-form-item>

              <el-form-item :label="t.autoStart">
                <el-switch v-model="mappingForm.auto_start" />
              </el-form-item>
              <el-form-item>
                <el-button type="primary" @click="addMapping">
                  {{ isEditingMapping ? t.save : t.create }}
                </el-button>
                <el-button @click="resetMappingForm" v-if="!isEditingMapping"
                  >{{ t.reset }}</el-button
                >
              </el-form-item>
            </el-form>
          </el-card>
        </el-col>

        <el-col :span="18">
          <el-card shadow="never" style="margin-bottom: 18px">
            <template #header>
              <div
                style="
                  display: flex;
                  justify-content: space-between;
                  align-items: center;
                "
              >
                <b>{{ t.trafficDashboard }}</b>
                <div style="display: flex; gap: 10px; align-items: center">
                  <span class="muted" style="font-size: 12px">{{ t.autoRefresh }}:</span>
                  <el-switch
                    v-model="trafficChartEnabled"
                    @change="toggleTrafficChart"
                  />
                  <el-input-number
                    v-model="trafficChartIntervalSeconds"
                    :min="1"
                    :max="60"
                    size="small"
                    style="width: 100px"
                    @change="updateTrafficChartInterval"
                  />
                  <span class="muted" style="font-size: 12px">{{ t.seconds }}</span>
                </div>
              </div>
            </template>
            <div ref="trafficChartEl" style="width: 100%; height: 260px"></div>
          </el-card>

          <el-card shadow="never">
            <template #header>
              <div
                style="
                  display: flex;
                  justify-content: space-between;
                  align-items: center;
                "
              >
                <b>{{ t.mappingList }}</b>
                <div style="display: flex; gap: 10px; align-items: center">
                  <el-switch
                    v-model="autoRefreshMappings"
                    :active-text="currentLang === 'zh' ? '自动刷新' : 'Auto Refresh'"
                    @change="toggleMappingAutoRefresh"
                    style="--el-switch-on-color: #13ce66"
                  />
                  <el-select
                    v-model="refreshInterval"
                    :placeholder="currentLang === 'zh' ? '刷新间隔' : 'Interval'"
                    style="width: 100px"
                    size="small"
                  >
                    <el-option :label="'5s'" :value="5000"></el-option>
                    <el-option :label="'10s'" :value="10000"></el-option>
                    <el-option :label="'30s'" :value="30000"></el-option>
                    <el-option :label="'60s'" :value="60000"></el-option>
                  </el-select>
                </div>
              </div>
            </template>
            <el-table
              :data="mappings"
              size="small"
              style="width: 100%"
              v-loading="loading.maps"
            >
              <el-table-column :label="t.port" width="90">
                <template #default="s">
                  <b style="color: #409eff">{{ s.row.local_port }}</b>
                </template>
              </el-table-column>

              <el-table-column :label="t.type" width="70">
                <template #default="s">
                  <el-tag v-if="s.row.type==='socks5'" type="warning" size="small">S5</el-tag>
                  <el-tag v-else-if="s.row.type==='http'" type="success" size="small">HTTP</el-tag>
                  <el-tag v-else size="small">TCP</el-tag>
                </template>
              </el-table-column>

              <el-table-column :label="t.autoStart" width="80">
                <template #default="s">
                  <el-switch
                    v-model="s.row.auto_start"
                    @change="toggleAutoStart(s.row)"
                    size="small"
                  />
                </template>
              </el-table-column>

              <el-table-column :label="t.remoteTarget" width="180">
                <template #default="s">
                  <span v-if="s.row.type==='socks5' || s.row.type==='http'" class="muted"><i>Dynamic</i></span>
                  <span v-else class="code" style="font-size: 12px">{{ s.row.remote_host }}:{{ s.row.remote_port }}</span>
                </template>
              </el-table-column>

              <el-table-column :label="t.chain" min-width="30%">
                <template #default="s">
                  <el-tag
                    v-for="n in s.row.chain"
                    :key="n"
                    effect="plain"
                    style="margin-right: 2px; zoom: 0.75"
                    >{{ n }}</el-tag
                  >
                </template>
              </el-table-column>

              <el-table-column :label="t.traffic" width="110">
                <template #default="s">
                  <div
                    v-if="s.row.running && trafficStats[s.row.id]"
                    class="traffic-info"
                    style="font-size: 12px"
                  >
                    <div>
                      <span class="traffic-label">↑</span>{{ formatBytes(trafficStats[s.row.id].up_bytes) }}
                    </div>
                    <div>
                      <span class="traffic-label">↓</span>{{ formatBytes(trafficStats[s.row.id].down_bytes) }}
                    </div>
                  </div>
                  <span v-else class="muted">-</span>
                </template>
              </el-table-column>

              <el-table-column :label="t.operation" width="160" fixed="right">
                <template #default="s">
                  <div style="display: flex; gap: 5px">
                    <el-button
                      size="small"
                      :type="s.row.running ? 'danger' : 'success'"
                      @click="s.row.running ? stopMapping(s.row.id) : startMapping(s.row.id)"
                      circle
                    >
                      <el-icon v-if="s.row.running"><Video-Pause /></el-icon>
                      <el-icon v-else><Video-Play /></el-icon>
                    </el-button>

                    <el-tooltip :content="t.edit" placement="top">
                      <el-button size="small" circle icon="Edit" @click="editMapping(s.row)" />
                    </el-tooltip>

                    <el-tooltip :content="t.copy" placement="top">
                      <el-button size="small" circle icon="DocumentCopy" @click="copyMapping(s.row)" />
                    </el-tooltip>

                    <el-popconfirm :title="t.deleteConfirm" @confirm="delMapping(s.row.id)">
                      <template #reference>
                        <el-button size="small" type="danger" circle icon="Delete" />
                      </template>
                    </el-popconfirm>
                  </div>
                </template>
              </el-table-column>
            </el-table>
          </el-card>
        </el-col>
      </el-row>
    </div>
  `,
};

