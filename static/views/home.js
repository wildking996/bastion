import { useUpdatePanel } from "../app/update_panel.js";

const { inject } = Vue;

export default {
  name: "ViewHome",
  setup() {
    const t = inject("t");
    const openConfirmDialog = inject("openConfirmDialog");
    const ElMessage = ElementPlus.ElMessage;

    return {
      t,
      ...useUpdatePanel({ t, openConfirmDialog, ElMessage }),
    };
  },
  template: `
    <div style="max-width: 900px; margin: 0 auto;">
      <el-row justify="space-between" align="middle" style="margin-bottom: 12px" :gutter="12">
        <el-col :span="24">
          <h2 style="margin: 0">{{ t.navHomeUpdates }}</h2>
        </el-col>
      </el-row>

      <el-card shadow="never">
        <template #header>
          <div style="display:flex;justify-content:space-between;align-items:center;gap:10px;flex-wrap:wrap;">
            <b>{{ t.update }}</b>
            <el-space wrap>
              <el-button type="primary" :loading="updateChecking" @click="checkForUpdate">{{ t.checkUpdate }}</el-button>
              <el-button type="warning" :disabled="!checked || !updateAvailable" @click="confirmUpdate">{{ t.confirmUpdate }}</el-button>
            </el-space>
          </div>
        </template>

        <div v-if="!checked" class="muted" style="margin-bottom: 12px;">
          {{ t.updateNotChecked }}
        </div>

        <div v-if="updateHelperLogPath" style="margin-bottom: 12px">
          <el-alert type="info" :closable="false" :title="t.updateHelperLog" show-icon>
            <template #default>
              <div class="code" style="word-break: break-all">{{ updateHelperLogPath }}</div>
            </template>
          </el-alert>
        </div>

        <el-row :gutter="12">
          <el-col :span="12">
            <div class="muted" style="font-size: 12px">{{ t.currentVersion }}</div>
            <div class="code" style="margin-top: 4px">{{ updateInfo.current_version || '-' }}</div>
          </el-col>
          <el-col :span="12">
            <div class="muted" style="font-size: 12px">{{ t.latestVersion }}</div>
            <div class="code" style="margin-top: 4px">{{ updateInfo.latest_version || '-' }}</div>
          </el-col>
        </el-row>

        <div style="margin-top: 12px">
          <el-alert v-if="updateInfo.update_available === true" type="success" :closable="false" :title="t.updateAvailable" show-icon></el-alert>
          <el-alert v-else-if="checked" type="info" :closable="false" :title="t.upToDate" show-icon></el-alert>
        </div>

        <div style="margin-top: 12px">
          <el-collapse v-model="updateProxyCollapse">
            <el-collapse-item name="proxy" :title="t.updateProxy">
              <el-form label-width="160px">
                <el-form-item :label="t.updateProxyDetected">
                  <el-input :model-value="updateProxyDetected || '-'" readonly></el-input>
                </el-form-item>
                <el-form-item :label="t.updateProxyManual">
                  <el-input v-model="updateProxyManual" :placeholder="t.updateProxyPlaceholder" clearable></el-input>
                </el-form-item>
                <el-form-item label=" ">
                  <el-button type="primary" :loading="updateProxySaving" @click="saveUpdateProxy">{{ t.updateProxySave }}</el-button>
                  <el-button :loading="updateProxySaving" @click="clearUpdateProxy">{{ t.updateProxyClear }}</el-button>
                </el-form-item>
              </el-form>
            </el-collapse-item>
          </el-collapse>
        </div>
      </el-card>
    </div>
  `,
};
