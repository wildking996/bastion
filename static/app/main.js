import { apiJSON } from "./api.js";
import {
  DEFAULT_ROUTE,
  ensureDefaultRoute,
  getCurrentRoute,
  navigate,
  onRouteChange,
} from "./router.js";
import { GROUPS, getViewByPath, getViewsByGroup } from "./view_registry.js";
import {
  loadGroupState,
  loadSidebarCollapsed,
  saveGroupState,
  saveSidebarCollapsed,
} from "./sidebar_state.js";

const { createApp, ref, shallowRef, computed, onMounted, onUnmounted, provide } =
  Vue;

function titleForRoute(route, lang) {
  if (route.startsWith("/logs/http")) {
    return lang === "zh" ? i18nConfig.zh.titleLogs : i18nConfig.en.titleLogs;
  }
  if (route.startsWith("/logs/errors")) {
    return lang === "zh" ? i18nConfig.zh.titleErrors : i18nConfig.en.titleErrors;
  }
  return lang === "zh" ? i18nConfig.zh.titleIndex : i18nConfig.en.titleIndex;
}

function makeGroupDefaults() {
  const defaults = {};
  for (const group of GROUPS) defaults[group.key] = true;
  return defaults;
}

const app = createApp({
  setup() {
    const ElMessage = ElementPlus.ElMessage;

    const currentLang = ref(i18n.getLanguage());
    const t = ref(i18n.getAll());

    const elLocale = computed(() => {
      if (currentLang.value === "zh") return window.ElementPlusLocaleZhCn;
      return window.ElementPlusLocaleEn;
    });

    const collapsed = ref(loadSidebarCollapsed());
    const asideWidth = computed(() => (collapsed.value ? "72px" : "260px"));

    const groupState = ref(loadGroupState(makeGroupDefaults()));
    const defaultOpenGroups = computed(() =>
      Object.entries(groupState.value)
        .filter(([, opened]) => opened)
        .map(([key]) => key)
    );

    const route = ref(DEFAULT_ROUTE);
    const activeComponent = shallowRef(null);
    const componentCache = new Map();

    const activeView = computed(() => getViewByPath(route.value));
    const activeGroup = computed(() => {
      const view = activeView.value;
      if (!view) return null;
      return GROUPS.find((g) => g.key === view.group) || null;
    });

    const viewTitle = computed(() => {
      const view = activeView.value;
      if (!view) return t.value.console;
      return t.value[view.titleKey] || t.value.console;
    });

    const breadcrumbItems = computed(() => {
      const items = [{ label: t.value.console }];
      if (activeGroup.value) {
        items.push({ label: t.value[activeGroup.value.titleKey] });
      }
      if (activeView.value) {
        items.push({ label: t.value[activeView.value.titleKey] });
      }
      return items;
    });

    const shutdownDialogVisible = ref(false);
    const shutdownCode = ref("");
    const shutdownExpiresAt = ref(0);
    const shutdownCodeExpiry = ref("");
    const inputCode = ref("");
    const generating = ref(false);
    const shuttingDown = ref(false);
    let shutdownExpiryTimer = null;

    const groupIcons = {
      manage: "Tools",
      logs: "Document",
      system: "Setting",
    };

    const viewIcons = {
      "/bastions": "Connection",
      "/mappings": "Share",
      "/logs/http": "Document",
      "/logs/errors": "Warning",
      "/system/update": "Download",
    };

    const stopShutdownTimer = () => {
      if (!shutdownExpiryTimer) return;
      clearInterval(shutdownExpiryTimer);
      shutdownExpiryTimer = null;
    };

    const updateShutdownCountdown = () => {
      const now = Math.floor(Date.now() / 1000);
      const left = Math.max(0, shutdownExpiresAt.value - now);
      if (left <= 0) {
        shutdownCodeExpiry.value = t.value.codeExpired;
        stopShutdownTimer();
        return;
      }
      const minutes = String(Math.floor(left / 60)).padStart(2, "0");
      const seconds = String(left % 60).padStart(2, "0");
      shutdownCodeExpiry.value = `${minutes}:${seconds}`;
    };

    const showShutdownDialog = () => {
      shutdownDialogVisible.value = true;
      shutdownCode.value = "";
      shutdownExpiresAt.value = 0;
      shutdownCodeExpiry.value = "";
      inputCode.value = "";
      stopShutdownTimer();
    };

    const generateShutdownCode = async () => {
      generating.value = true;
      try {
        const data = await apiJSON("/shutdown/generate-code", "POST");
        shutdownCode.value = data.code || "";
        shutdownExpiresAt.value = Number(data.expires_at || 0);
        inputCode.value = "";

        updateShutdownCountdown();
        stopShutdownTimer();
        shutdownExpiryTimer = setInterval(updateShutdownCountdown, 1000);

        ElMessage.success(t.value.codeGenerated);
      } catch (e) {
        ElMessage.error(e.message);
      } finally {
        generating.value = false;
      }
    };

    const verifyAndShutdown = async () => {
      if ((inputCode.value || "").length !== 6) {
        ElMessage.warning(t.value.enterCode);
        return;
      }
      shuttingDown.value = true;
      try {
        await apiJSON("/shutdown/verify", "POST", { code: inputCode.value });
        ElMessage.success(t.value.shutdownInitiated);
        shutdownDialogVisible.value = false;
        stopShutdownTimer();
      } catch (e) {
        ElMessage.error(e.message);
      } finally {
        shuttingDown.value = false;
      }
    };

    const refreshPage = () => window.location.reload();

    const switchLanguage = (lang) => {
      if (!lang || lang === currentLang.value) return;
      i18n.setLanguage(lang);
      currentLang.value = lang;
      t.value = i18n.getAll();
      document.title = titleForRoute(route.value, currentLang.value);
    };

    const toggleSidebar = () => {
      collapsed.value = !collapsed.value;
      saveSidebarCollapsed(collapsed.value);
    };

    const onOpenGroup = (key) => {
      groupState.value[key] = true;
      saveGroupState(groupState.value);
    };

    const onCloseGroup = (key) => {
      groupState.value[key] = false;
      saveGroupState(groupState.value);
    };

    const setRoute = (nextRoute) => {
      const view = getViewByPath(nextRoute);
      if (!view) {
        navigate(DEFAULT_ROUTE);
        return;
      }

      route.value = view.path;
      if (!componentCache.has(view.path)) {
        componentCache.set(view.path, Vue.defineAsyncComponent(view.loader));
      }
      activeComponent.value = componentCache.get(view.path);
      document.title = titleForRoute(route.value, currentLang.value);
    };

    const onSelectMenu = (index) => {
      if (!index || typeof index !== "string") return;
      if (index.startsWith("/")) navigate(index);
    };

    const viewsByGroup = computed(() => {
      const grouped = {};
      for (const group of GROUPS) grouped[group.key] = getViewsByGroup(group.key);
      return grouped;
    });

    provide("t", t);
    provide("currentLang", currentLang);
    provide("navigate", navigate);

    let stopRouteListener = null;
    onMounted(() => {
      ensureDefaultRoute();
      setRoute(getCurrentRoute());
      stopRouteListener = onRouteChange(setRoute);
      document.title = titleForRoute(route.value, currentLang.value);
    });

    onUnmounted(() => {
      if (stopRouteListener) stopRouteListener();
      stopShutdownTimer();
    });

    return {
      elLocale,
      currentLang,
      t,
      collapsed,
      asideWidth,
      viewTitle,
      breadcrumbItems,
      groups: GROUPS,
      groupIcons,
      viewIcons,
      viewsByGroup,
      defaultOpenGroups,
      route,
      activeComponent,
      onSelectMenu,
      onOpenGroup,
      onCloseGroup,
      refreshPage,
      switchLanguage,
      toggleSidebar,
      shutdownDialogVisible,
      shutdownCode,
      shutdownCodeExpiry,
      inputCode,
      generating,
      shuttingDown,
      showShutdownDialog,
      generateShutdownCode,
      verifyAndShutdown,
    };
  },
  template: `
    <el-config-provider :locale="elLocale">
      <div class="app-shell">
        <el-container>
          <el-header class="app-header" height="auto">
            <el-row justify="space-between" align="middle" :gutter="10">
              <el-col :span="14" style="min-width: 240px">
                <div style="display:flex;align-items:center;gap:10px;">
                  <el-button
                    :icon="collapsed ? 'Expand' : 'Fold'"
                    circle
                    @click="toggleSidebar"
                    :title="collapsed ? 'Expand' : 'Collapse'"
                  ></el-button>
                  <div class="brand">
                    <div class="brand-title">{{ t.console }}</div>
                    <el-breadcrumb separator="/" class="brand-subtitle">
                      <el-breadcrumb-item v-for="(b, i) in breadcrumbItems" :key="i">{{ b.label }}</el-breadcrumb-item>
                    </el-breadcrumb>
                  </div>
                </div>
              </el-col>
              <el-col :span="10" style="display:flex;justify-content:flex-end">
                <el-space wrap>
                  <el-radio-group v-model="currentLang" size="small" @change="switchLanguage">
                    <el-radio-button label="zh">中文</el-radio-button>
                    <el-radio-button label="en">English</el-radio-button>
                  </el-radio-group>
                  <el-button icon="Refresh" circle @click="refreshPage"></el-button>
                  <el-button icon="SwitchButton" type="danger" @click="showShutdownDialog">{{ t.shutdown }}</el-button>
                </el-space>
              </el-col>
            </el-row>
          </el-header>

          <el-container class="app-body">
            <el-aside :width="asideWidth" class="app-aside">
              <el-card class="aside-card" shadow="never">
                <el-scrollbar height="100%">
                  <el-menu
                    :default-active="route"
                    :default-openeds="defaultOpenGroups"
                    :collapse="collapsed"
                    class="aside-menu"
                    @select="onSelectMenu"
                    @open="onOpenGroup"
                    @close="onCloseGroup"
                  >
                    <el-sub-menu v-for="g in groups" :key="g.key" :index="g.key">
                      <template #title>
                        <el-icon>
                          <component :is="groupIcons[g.key]" />
                        </el-icon>
                        <span>{{ t[g.titleKey] }}</span>
                      </template>
                      <el-menu-item
                        v-for="v in viewsByGroup[g.key]"
                        :key="v.path"
                        :index="v.path"
                      >
                        <el-icon v-if="viewIcons[v.path]">
                          <component :is="viewIcons[v.path]" />
                        </el-icon>
                        <span>{{ t[v.titleKey] }}</span>
                      </el-menu-item>
                    </el-sub-menu>
                  </el-menu>
                </el-scrollbar>
              </el-card>
            </el-aside>

            <el-main class="app-main">
              <div class="main-surface">
                <keep-alive>
                  <component :is="activeComponent" />
                </keep-alive>
              </div>
            </el-main>
          </el-container>
        </el-container>

        <el-dialog v-model="shutdownDialogVisible" :title="t.shutdown" width="500px">
          <div style="margin-bottom: 20px">
            <el-alert :title="t.shutdownConfirm" type="warning" :closable="false"></el-alert>
          </div>

          <div v-if="!shutdownCode" style="text-align: center">
            <el-button type="primary" @click="generateShutdownCode" :loading="generating">
              {{ t.generateCode }}
            </el-button>
          </div>

          <div v-else>
            <el-descriptions :column="1" border>
              <el-descriptions-item :label="t.confirmationCode">
                <span class="code" style="font-size: 24px; font-weight: 700; color: var(--el-color-primary)">{{ shutdownCode }}</span>
              </el-descriptions-item>
              <el-descriptions-item :label="t.expiresIn">
                <el-tag type="info">{{ shutdownCodeExpiry }}</el-tag>
              </el-descriptions-item>
            </el-descriptions>

            <el-form style="margin-top: 20px" @submit.prevent="verifyAndShutdown">
              <el-form-item :label="t.enterCode">
                <el-input v-model="inputCode" maxlength="6" :placeholder="t.enterCode" clearable></el-input>
              </el-form-item>
              <el-form-item>
                <el-button
                  type="danger"
                  @click="verifyAndShutdown"
                  :loading="shuttingDown"
                  :disabled="(inputCode || '').length !== 6"
                >
                  {{ t.shutdownSystem }}
                </el-button>
                <el-button @click="shutdownDialogVisible = false">{{ t.cancel }}</el-button>
              </el-form-item>
            </el-form>
          </div>
        </el-dialog>
      </div>
    </el-config-provider>
  `,
});

for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component);
}

app.use(ElementPlus).mount("#app");
