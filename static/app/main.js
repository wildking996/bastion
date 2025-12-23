import {
  DEFAULT_ROUTE,
  ensureDefaultRoute,
  getCurrentRoute,
  navigate,
  onRouteChange,
} from "./router.js";
import { GROUPS, getViewByPath, getViewsByGroup } from "./view_registry.js";
import { loadSidebarCollapsed, saveSidebarCollapsed } from "./sidebar_state.js";

const {
  createApp,
  ref,
  shallowRef,
  computed,
  onMounted,
  onUnmounted,
  provide,
  nextTick,
} = Vue;

function titleForRoute(route, lang) {
  if (route.startsWith("/logs/http")) {
    return lang === "zh" ? i18nConfig.zh.titleLogs : i18nConfig.en.titleLogs;
  }
  if (route.startsWith("/logs/errors")) {
    return lang === "zh" ? i18nConfig.zh.titleErrors : i18nConfig.en.titleErrors;
  }
  return lang === "zh" ? i18nConfig.zh.titleIndex : i18nConfig.en.titleIndex;
}

const app = createApp({
  setup() {
    const currentLang = ref(i18n.getLanguage());
    const t = ref(i18n.getAll());

    const elLocale = computed(() => {
      if (currentLang.value === "zh") return window.ElementPlusLocaleZhCn;
      return window.ElementPlusLocaleEn;
    });

    const collapsed = ref(loadSidebarCollapsed());
    const asideWidth = computed(() => (collapsed.value ? "72px" : "260px"));

    const route = ref(DEFAULT_ROUTE);
    const activeComponent = shallowRef(null);
    const componentCache = new Map();

    const menuRef = ref(null);

    const activeView = computed(() => getViewByPath(route.value));
    const activeGroup = computed(() => {
      const view = activeView.value;
      if (!view) return null;
      return GROUPS.find((g) => g.key === view.group) || null;
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
      "/system/update": "Setting",
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

    const syncOpenGroups = async () => {
      await nextTick();
      const menu = menuRef.value;
      if (!menu) return;

      const keep = activeGroup.value ? activeGroup.value.key : "";
      for (const g of GROUPS) {
        if (g.key === keep) {
          if (typeof menu.open === "function") menu.open(g.key);
        } else {
          if (typeof menu.close === "function") menu.close(g.key);
        }
      }
    };

    const setRoute = async (nextRoute) => {
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

      await syncOpenGroups();
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
    onMounted(async () => {
      ensureDefaultRoute();
      await setRoute(getCurrentRoute());
      stopRouteListener = onRouteChange(setRoute);
      document.title = titleForRoute(route.value, currentLang.value);
    });

    onUnmounted(() => {
      if (stopRouteListener) stopRouteListener();
    });

    return {
      elLocale,
      currentLang,
      t,
      collapsed,
      asideWidth,
      breadcrumbItems,
      groups: GROUPS,
      groupIcons,
      viewIcons,
      viewsByGroup,
      route,
      activeComponent,
      menuRef,
      onSelectMenu,
      refreshPage,
      switchLanguage,
      toggleSidebar,
    };
  },
  template: `
    <el-config-provider :locale="elLocale">
      <el-container class="app-shell">
        <el-header class="app-header" height="auto">
          <el-row justify="space-between" align="middle" :gutter="10">
            <el-col :span="14" style="min-width: 240px">
              <div style="display:flex;align-items:center;gap:10px;">
                <el-button
                  :icon="collapsed ? 'Expand' : 'Fold'"
                  circle
                  @click="toggleSidebar"
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
              </el-space>
            </el-col>
          </el-row>
        </el-header>

        <el-container class="app-body">
          <el-aside :width="asideWidth" class="app-aside">
            <el-menu
              ref="menuRef"
              :default-active="route"
              :collapse="collapsed"
              class="aside-menu"
              @select="onSelectMenu"
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
          </el-aside>

          <el-main class="app-main">
            <el-scrollbar class="main-scroll">
              <div class="main-inner">
                <keep-alive>
                  <component :is="activeComponent" />
                </keep-alive>
              </div>
            </el-scrollbar>
          </el-main>
        </el-container>
      </el-container>
    </el-config-provider>
  `,
});

for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component);
}

app.use(ElementPlus).mount("#app");
