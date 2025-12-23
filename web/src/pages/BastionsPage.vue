<template>
  <div class="app-page">
    <el-card class="app-card">
      <template #header>
        <div class="card-header">
          <span>{{ t("bastions.title") }}</span>
          <el-button type="primary" @click="openAdd">{{ t("common.add") }}</el-button>
        </div>
      </template>

      <el-table :data="paged" stripe v-loading="loading">
        <el-table-column prop="id" :label="t('table.id')" width="90" />
        <el-table-column prop="name" :label="t('bastions.name')" min-width="180" />
        <el-table-column prop="host" :label="t('bastions.host')" min-width="180" />
        <el-table-column prop="port" :label="t('bastions.port')" width="100" />
        <el-table-column prop="username" :label="t('bastions.username')" min-width="140" />
        <el-table-column :label="t('table.actions')" width="220">
          <template #default="scope">
            <el-button size="small" @click="openEdit(scope.row)">{{ t("common.edit") }}</el-button>
            <el-button size="small" @click="openCopy(scope.row)">{{ t("common.copy") }}</el-button>
            <el-popconfirm
              :title="t('dialogs.deleteTitle')"
              @confirm="remove(scope.row)"
            >
              <template #reference>
                <el-button size="small" type="danger">{{ t("common.delete") }}</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>

      <UnifiedPagination v-model:page="page" v-model:pageSize="pageSize" :total="list.length" />
    </el-card>

    <FormDialog v-model="dialogVisible" :title="dialogTitle">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="110px">
        <el-form-item prop="name" :label="t('bastions.name')">
          <el-input v-model="form.name" :disabled="isEdit"  :placeholder="t('common.optional')" />
        </el-form-item>
        <el-form-item prop="host" :label="t('bastions.host')">
          <el-input v-model="form.host" :disabled="isEdit" />
        </el-form-item>
        <el-form-item prop="port" :label="t('bastions.port')">
          <el-input-number v-model="form.port" :disabled="isEdit" :min="1" :max="65535" />
        </el-form-item>
        <el-form-item prop="username" :label="t('bastions.username')">
          <el-input v-model="form.username" />
        </el-form-item>
        <el-form-item prop="password" :label="t('bastions.password')">
          <el-input v-model="form.password" show-password />
        </el-form-item>
        <el-form-item prop="pkey_path" :label="t('bastions.pkeyPath')">
          <el-input v-model="form.pkey_path" />
        </el-form-item>
        <el-form-item prop="pkey_passphrase" :label="t('bastions.pkeyPassphrase')">
          <el-input v-model="form.pkey_passphrase" show-password />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">{{ t("common.cancel") }}</el-button>
        <el-button type="primary" :loading="saving" @click="save">{{ t("common.confirm") }}</el-button>
      </template>
    </FormDialog>
  </div>
</template>

<script setup lang="ts">
import type { FormInstance, FormRules } from "element-plus";
import { computed, onMounted, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";

import { api } from "@/api/client";
import type { Bastion, BastionCreate } from "@/api/types";
import FormDialog from "@/components/FormDialog.vue";
import UnifiedPagination from "@/components/UnifiedPagination.vue";
import { requiredNumberRule, requiredTrimRule } from "@/utils/formRules";

const { t, locale } = useI18n();

const loading = ref(false);
const saving = ref(false);
const list = ref<Bastion[]>([]);

const page = ref(1);
const pageSize = ref(20);

const paged = computed(() => {
  const start = (page.value - 1) * pageSize.value;
  return list.value.slice(start, start + pageSize.value);
});

type Mode = "add" | "edit" | "copy";
const mode = ref<Mode>("add");

const dialogVisible = ref(false);
function joinTitle(action: string, subject: string) {
  return locale.value.startsWith("en") ? action + " " + subject : action + subject;
}

const dialogTitle = computed(() => {
  const subject = t("bastions.title");
  if (mode.value === "add") return joinTitle(t("common.add"), subject);
  if (mode.value === "edit") return joinTitle(t("common.edit"), subject);
  return joinTitle(t("common.copy"), subject);
});

const isEdit = computed(() => mode.value === "edit");

const formRef = ref<FormInstance>();
const form = reactive<
  BastionCreate & {
    id?: number;
  }
>({
  name: "",
  host: "",
  port: 22,
  username: "",
  password: "",
  pkey_path: "",
  pkey_passphrase: "",
});

const rules = computed<FormRules>(() => {
  const base: FormRules = {
    host: [requiredTrimRule(t, t("bastions.host"))],
    port: [requiredNumberRule(t, t("bastions.port"))],
    username: [requiredTrimRule(t, t("bastions.username"))],
  };

  if (mode.value !== "edit") {
    base.password = [requiredTrimRule(t, t("bastions.password"))];
  }

  return base;
});


async function refresh() {
  loading.value = true;
  try {
    const res = await api.get<Bastion[]>("/bastions");
    list.value = res.data;
  } finally {
    loading.value = false;
  }
}

function openAdd() {
  mode.value = "add";
  Object.assign(form, {
    id: undefined,
    name: "",
    host: "",
    port: 22,
    username: "",
    password: "",
    pkey_path: "",
    pkey_passphrase: "",
  });
  dialogVisible.value = true;
}

function openEdit(row: Bastion) {
  mode.value = "edit";
  Object.assign(form, {
    id: row.id,
    name: row.name,
    host: row.host,
    port: row.port,
    username: row.username,
    password: "",
    pkey_path: row.pkey_path ?? "",
    pkey_passphrase: "",
  });
  dialogVisible.value = true;
}

function openCopy(row: Bastion) {
  mode.value = "copy";
  Object.assign(form, {
    id: undefined,
    name: `${row.name}-copy`,
    host: row.host,
    port: row.port,
    username: row.username,
    password: "",
    pkey_path: row.pkey_path ?? "",
    pkey_passphrase: "",
  });
  dialogVisible.value = true;
}

async function save() {
  const ok = await formRef.value?.validate().catch(() => false);
  if (!ok) return;

  saving.value = true;
  try {
    const payload: BastionCreate = {
      name: form.name?.trim() || "",
      host: form.host.trim(),
      port: form.port,
      username: form.username.trim(),
      password: form.password?.trim() || "",
      pkey_path: form.pkey_path?.trim() || "",
      pkey_passphrase: form.pkey_passphrase?.trim() || "",
    };

    if (mode.value === "edit") {
      await api.put(`/bastions/${form.id}`, payload);
    } else {
      await api.post(`/bastions`, payload);
    }

    dialogVisible.value = false;
    await refresh();
  } finally {
    saving.value = false;
  }
}

async function remove(row: Bastion) {
  await api.delete(`/bastions/${row.id}`);
  await refresh();
}

onMounted(() => {
  refresh().catch(() => undefined);
});
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
