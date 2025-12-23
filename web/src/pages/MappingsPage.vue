<template>
  <div class="app-page">
    <el-card class="app-card">
      <template #header>
        <div class="card-header">
          <span>{{ t("mappings.title") }}</span>
          <el-button type="primary" @click="openAdd">{{ t("common.add") }}</el-button>
        </div>
      </template>

      <el-table :data="paged" stripe v-loading="loading">
        <el-table-column prop="id" :label="t('mappings.id')" min-width="200" />
        <el-table-column prop="type" :label="t('mappings.type')" width="90" />
        <el-table-column :label="t('mappings.local')" min-width="160">
          <template #default="scope">
            <span class="mono">{{ scope.row.local_host }}:{{ scope.row.local_port }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('mappings.remote')" min-width="160">
          <template #default="scope">
            <span class="mono">{{ scope.row.remote_host }}:{{ scope.row.remote_port }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('mappings.chain')" min-width="200">
          <template #default="scope">
            <el-tag v-for="b in scope.row.chain" :key="b" size="small" style="margin-right:6px">{{ b }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('mappings.autoStart')" width="90">
          <template #default="scope">
            <el-tag :type="scope.row.auto_start ? 'success' : 'info'">
              {{ scope.row.auto_start ? t("common.yes") : t("common.no") }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('mappings.running')" width="100">
          <template #default="scope">
            <el-tag :type="scope.row.running ? 'success' : 'info'">
              {{ scope.row.running ? t("common.on") : t("common.off") }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column :label="t('table.actions')" width="420">
          <template #default="scope">
            <el-button
              size="small"
              :type="scope.row.running ? 'warning' : 'success'"
              @click="toggleRun(scope.row)"
            >
              {{ scope.row.running ? t('mappings.stop') : t('mappings.start') }}
            </el-button>
            <el-button size="small" @click="openEdit(scope.row)">{{ t('common.edit') }}</el-button>
            <el-button size="small" @click="openCopyModify(scope.row)">{{ t('mappings.copyModify') }}</el-button>
            <el-button size="small" @click="openTraffic(scope.row)">{{ t('mappings.trafficChart') }}</el-button>
            <el-popconfirm :title="t('dialogs.deleteTitle')" @confirm="remove(scope.row)">
              <template #reference>
                <el-button size="small" type="danger" :disabled="scope.row.running">{{ t('common.delete') }}</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>

      <UnifiedPagination v-model:page="page" v-model:pageSize="pageSize" :total="list.length" />
    </el-card>

    <FormDialog v-model="dialogVisible" :title="dialogTitle">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="110px">
        <el-form-item prop="id" :label="t('mappings.id')">
          <el-input v-model="form.id" :disabled="isEdit" :placeholder="t('common.optional')" />
        </el-form-item>

        <el-form-item prop="type" :label="t('mappings.type')">
          <el-select v-model="form.type" :disabled="isEdit">
            <el-option label="tcp" value="tcp" />
            <el-option label="socks5" value="socks5" />
            <el-option label="http" value="http" />
          </el-select>
        </el-form-item>

        <el-form-item prop="local_host" :label="t('mappings.localHost')">
          <el-input v-model="form.local_host" :disabled="isEdit" />
        </el-form-item>
        <el-form-item prop="local_port" :label="t('mappings.localPort')">
          <el-input-number v-model="form.local_port" :disabled="isEdit" :min="1" :max="65535" />
        </el-form-item>

        <template v-if="form.type === 'tcp'">
          <el-form-item prop="remote_host" :label="t('mappings.remoteHost')">
            <el-input v-model="form.remote_host" :disabled="isEdit" />
          </el-form-item>
          <el-form-item prop="remote_port" :label="t('mappings.remotePort')">
            <el-input-number v-model="form.remote_port" :disabled="isEdit" :min="1" :max="65535" />
          </el-form-item>
        </template>

        <el-form-item :label="t('mappings.chain')" prop="chain">
          <el-select
            v-model="form.chain"
            multiple
            filterable
            allow-create
            default-first-option
            style="width: 100%"
          >
            <el-option v-for="b in bastionNames" :key="b" :label="b" :value="b" />
          </el-select>
        </el-form-item>

        <el-form-item :label="t('mappings.allowCidrs')" prop="allow_cidrs">
          <el-select
            v-model="form.allow_cidrs"
            multiple
            filterable
            allow-create
            default-first-option
            style="width: 100%"
            placeholder="e.g. 10.0.0.0/8"
          />
        </el-form-item>

        <el-form-item :label="t('mappings.denyCidrs')" prop="deny_cidrs">
          <el-select
            v-model="form.deny_cidrs"
            multiple
            filterable
            allow-create
            default-first-option
            style="width: 100%"
            placeholder="e.g. 0.0.0.0/0"
          />
        </el-form-item>

        <el-form-item :label="t('mappings.autoStart')" prop="auto_start">
          <el-switch v-model="form.auto_start" />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">{{ t("common.cancel") }}</el-button>
        <el-button type="primary" :loading="saving" @click="save">{{ t("common.confirm") }}</el-button>
      </template>
    </FormDialog>

    <el-dialog v-model="trafficVisible" :title="trafficTitle" width="880px" @closed="closeTraffic">
      <div ref="chartEl" style="height: 360px" />
      <el-divider />
      <el-descriptions :column="3" border>
        <el-descriptions-item :label="t('mappings.trafficUp')">{{ formatBytes(latestStats?.up_bytes ?? 0) }}</el-descriptions-item>
        <el-descriptions-item :label="t('mappings.trafficDown')">{{ formatBytes(latestStats?.down_bytes ?? 0) }}</el-descriptions-item>
        <el-descriptions-item :label="t('mappings.trafficConnections')">{{ latestStats?.connections ?? 0 }}</el-descriptions-item>
      </el-descriptions>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ElMessage } from "element-plus";
import type { FormInstance, FormRules } from "element-plus";
import * as echarts from "echarts";
import { computed, nextTick, onMounted, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";

import { api } from "@/api/client";
import type { Bastion, MappingCreate, MappingRead, StatsMap, StatsSnapshot } from "@/api/types";
import FormDialog from "@/components/FormDialog.vue";
import UnifiedPagination from "@/components/UnifiedPagination.vue";
import { requiredNumberRule, requiredTrimRule } from "@/utils/formRules";
import { formatBytes } from "@/utils/format";

const { t, locale } = useI18n();

const loading = ref(false);
const saving = ref(false);
const list = ref<MappingRead[]>([]);
const bastionNames = ref<string[]>([]);

const page = ref(1);
const pageSize = ref(20);

const paged = computed(() => {
  const start = (page.value - 1) * pageSize.value;
  return list.value.slice(start, start + pageSize.value);
});

type Mode = "add" | "edit" | "copyModify";
const mode = ref<Mode>("add");

const dialogVisible = ref(false);
function joinTitle(action: string, subject: string) {
  return locale.value.startsWith("en") ? action + " " + subject : action + subject;
}

const dialogTitle = computed(() => {
  const subject = t("mappings.title");
  if (mode.value === "add") return joinTitle(t("common.add"), subject);
  if (mode.value === "edit") return joinTitle(t("common.edit"), subject);
  return joinTitle(t("mappings.copyModify"), subject);
});

const isEdit = computed(() => mode.value === "edit");

const formRef = ref<FormInstance>();
const form = reactive<Required<MappingCreate>>({
  id: "",
  local_host: "127.0.0.1",
  local_port: 0,
  remote_host: "",
  remote_port: 0,
  chain: [],
  allow_cidrs: [],
  deny_cidrs: [],
  type: "tcp",
  auto_start: false,
});

const rules = computed<FormRules>(() => {
  const base: FormRules = {
    type: [requiredTrimRule(t, t("mappings.type"), "change")],
    local_host: [requiredTrimRule(t, t("mappings.localHost"))],
    local_port: [requiredNumberRule(t, t("mappings.localPort"))],
  };

  if (form.type === "tcp") {
    base.remote_host = [requiredTrimRule(t, t("mappings.remoteHost"))];
    base.remote_port = [requiredNumberRule(t, t("mappings.remotePort"))];
  }

  return base;
});


async function refresh() {
  loading.value = true;
  try {
    const res = await api.get<MappingRead[]>("/mappings");
    list.value = res.data;
  } finally {
    loading.value = false;
  }
}

async function loadBastions() {
  const res = await api.get<Bastion[]>("/bastions");
  bastionNames.value = res.data.map((b) => b.name);
}

function openAdd() {
  mode.value = "add";
  Object.assign(form, {
    id: "",
    local_host: "127.0.0.1",
    local_port: 0,
    remote_host: "",
    remote_port: 0,
    chain: [],
    allow_cidrs: [],
    deny_cidrs: [],
    type: "tcp",
    auto_start: false,
  });
  dialogVisible.value = true;
}

function openEdit(row: MappingRead) {
  mode.value = "edit";
  Object.assign(form, {
    id: row.id,
    local_host: row.local_host,
    local_port: row.local_port,
    remote_host: row.remote_host,
    remote_port: row.remote_port,
    chain: [...(row.chain ?? [])],
    allow_cidrs: [...(row.allow_cidrs ?? [])],
    deny_cidrs: [...(row.deny_cidrs ?? [])],
    type: row.type,
    auto_start: row.auto_start,
  });
  dialogVisible.value = true;
}

function openCopyModify(row: MappingRead) {
  mode.value = "copyModify";
  Object.assign(form, {
    id: "",
    local_host: row.local_host,
    local_port: row.local_port,
    remote_host: row.remote_host,
    remote_port: row.remote_port,
    chain: [...(row.chain ?? [])],
    allow_cidrs: [...(row.allow_cidrs ?? [])],
    deny_cidrs: [...(row.deny_cidrs ?? [])],
    type: row.type,
    auto_start: row.auto_start,
  });
  dialogVisible.value = true;
}

async function save() {
  const ok = await formRef.value?.validate().catch(() => false);
  if (!ok) return;

  saving.value = true;
  try {
    const payload: MappingCreate = {
      id: form.id.trim() || "",
      local_host: form.local_host.trim() || "127.0.0.1",
      local_port: form.local_port,
      remote_host: form.type === "tcp" ? form.remote_host.trim() : "",
      remote_port: form.type === "tcp" ? form.remote_port : 0,
      chain: (form.chain ?? []).map((v) => v.trim()).filter(Boolean),
      allow_cidrs: (form.allow_cidrs ?? []).map((v) => v.trim()).filter(Boolean),
      deny_cidrs: (form.deny_cidrs ?? []).map((v) => v.trim()).filter(Boolean),
      type: form.type,
      auto_start: form.auto_start,
    };

    if (payload.type === "tcp" && (!payload.remote_host || !payload.remote_port)) {
      ElMessage.error(t("validation.tcpRemoteRequired"));
      return;
    }

    if (mode.value === "edit") {
      await api.put(`/mappings/${encodeURIComponent(form.id)}`, payload);
    } else {
      await api.post(`/mappings`, payload);
    }

    dialogVisible.value = false;
    await refresh();
  } finally {
    saving.value = false;
  }
}

async function toggleRun(row: MappingRead) {
  if (row.running) await api.post(`/mappings/${encodeURIComponent(row.id)}/stop`);
  else await api.post(`/mappings/${encodeURIComponent(row.id)}/start`);
  await refresh();
}

async function remove(row: MappingRead) {
  await api.delete(`/mappings/${encodeURIComponent(row.id)}`);
  await refresh();
}

const trafficVisible = ref(false);
const trafficMappingId = ref<string>("");
const trafficTitle = computed(() => `${t("mappings.trafficChart")}: ${trafficMappingId.value}`);

const chartEl = ref<HTMLDivElement | null>(null);
let chart: echarts.ECharts | null = null;
let pollTimer: number | null = null;

const xAxis = ref<string[]>([]);
const upSeries = ref<number[]>([]);
const downSeries = ref<number[]>([]);
const latestStats = ref<StatsSnapshot | null>(null);

function pushPoint(stats: StatsSnapshot) {
  const label = new Date().toLocaleTimeString();
  xAxis.value.push(label);
  upSeries.value.push(stats.up_bytes);
  downSeries.value.push(stats.down_bytes);

  const maxPoints = 60;
  if (xAxis.value.length > maxPoints) {
    xAxis.value.splice(0, xAxis.value.length - maxPoints);
    upSeries.value.splice(0, upSeries.value.length - maxPoints);
    downSeries.value.splice(0, downSeries.value.length - maxPoints);
  }
}

function renderChart() {
  if (!chart) return;
  chart.setOption({
    tooltip: { trigger: "axis" },
    legend: { data: ["up", "down"], textStyle: { color: "#cbd5e1" } },
    grid: { left: 40, right: 20, top: 30, bottom: 30 },
    xAxis: {
      type: "category",
      data: xAxis.value,
      axisLabel: { color: "#94a3b8" },
    },
    yAxis: {
      type: "value",
      axisLabel: { color: "#94a3b8" },
      splitLine: { lineStyle: { color: "rgba(255,255,255,0.06)" } },
    },
    series: [
      { name: "up", type: "line", smooth: true, showSymbol: false, data: upSeries.value },
      { name: "down", type: "line", smooth: true, showSymbol: false, data: downSeries.value },
    ],
  });
}

async function pollStats() {
  const res = await api.get<StatsMap>("/stats");
  const s = res.data[trafficMappingId.value];
  if (!s) return;
  latestStats.value = s;
  pushPoint(s);
  renderChart();
}

async function openTraffic(row: MappingRead) {
  trafficMappingId.value = row.id;
  xAxis.value = [];
  upSeries.value = [];
  downSeries.value = [];
  latestStats.value = null;
  trafficVisible.value = true;

  await nextTick();
  if (chartEl.value) {
    chart = echarts.init(chartEl.value);
    renderChart();
  }

  await pollStats().catch(() => undefined);
  pollTimer = window.setInterval(() => {
    pollStats().catch(() => undefined);
  }, 1000);
}

function closeTraffic() {
  if (pollTimer != null) {
    window.clearInterval(pollTimer);
    pollTimer = null;
  }
  if (chart) {
    chart.dispose();
    chart = null;
  }
}

onMounted(() => {
  Promise.all([refresh(), loadBastions()]).catch(() => undefined);
});
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
