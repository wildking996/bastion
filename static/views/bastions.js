import { apiDELETE, apiPOST, apiPUT } from "../app/api.js";

const { ref, inject, onMounted } = Vue;

const PAGINATION_LAYOUT = "total, sizes, prev, pager, next, jumper";
const PAGE_SIZES = [10, 20, 50, 100];

export default {
  name: "ViewBastions",
  setup() {
    const t = inject("t");
    const ElMessage = ElementPlus.ElMessage;

    const loading = ref(false);
    const rows = ref([]);

    const page = ref(1);
    const pageSize = ref(10);
    const total = ref(0);

    const dialogVisible = ref(false);
    const dialogMode = ref("create");

    const defaultForm = {
      name: "",
      host: "",
      port: 22,
      username: "root",
      password: "",
      pkey_path: "",
      pkey_passphrase: "",
    };
    const form = ref({ ...defaultForm });
    const editingID = ref(null);

    const loadPage = async () => {
      loading.value = true;
      try {
        const params = new URLSearchParams();
        params.set("page", String(page.value));
        params.set("page_size", String(pageSize.value));

        const resp = await fetch(`/api/bastions?${params.toString()}`);
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

    const openEdit = (row) => {
      dialogMode.value = "edit";
      editingID.value = row.id;
      form.value = {
        name: row.name || "",
        host: row.host || "",
        port: row.port || 22,
        username: row.username || "",
        password: "",
        pkey_path: row.pkey_path || "",
        pkey_passphrase: row.pkey_passphrase || "",
      };
      dialogVisible.value = true;
    };

    const openCopy = (row) => {
      dialogMode.value = "create";
      editingID.value = null;
      form.value = {
        name: "",
        host: row.host || "",
        port: row.port || 22,
        username: row.username || "root",
        password: "",
        pkey_path: row.pkey_path || "",
        pkey_passphrase: row.pkey_passphrase || "",
      };
      dialogVisible.value = true;
    };

    const closeDialog = () => {
      dialogVisible.value = false;
    };

    const submit = async () => {
      try {
        if (!form.value.host) {
          ElMessage.warning(t.value.host);
          return;
        }
        if (!form.value.username) {
          ElMessage.warning(t.value.username);
          return;
        }

        if (dialogMode.value === "edit" && editingID.value) {
          await apiPUT(`/bastions/${editingID.value}`, form.value);
          ElMessage.success(t.value.editSuccess);
        } else {
          await apiPOST("/bastions", form.value);
          ElMessage.success(t.value.addSuccess);
        }

        closeDialog();
        await loadPage();
      } catch (e) {
        ElMessage.error(e.message);
      }
    };

    const del = async (row) => {
      try {
        await apiDELETE(`/bastions/${row.id}`);
        ElMessage.success(t.value.deleted);
        if (rows.value.length === 1 && page.value > 1) page.value -= 1;
        await loadPage();
      } catch (e) {
        ElMessage.error(e.message);
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

    return {
      t,
      loading,
      rows,
      page,
      pageSize,
      total,
      dialogVisible,
      dialogMode,
      form,
      openCreate,
      openEdit,
      openCopy,
      closeDialog,
      submit,
      del,
      PAGINATION_LAYOUT,
      PAGE_SIZES,
      handleSizeChange,
      handleCurrentChange,
    };
  },
  template: `
    <div>
      <el-row justify="space-between" align="middle" style="margin-bottom: 12px" :gutter="12">
        <el-col :span="12">
          <h2 style="margin: 0">{{ t.navBastions }}</h2>
        </el-col>
        <el-col :span="12" style="display:flex;justify-content:flex-end">
          <el-space wrap>
            <el-button type="primary" icon="Plus" @click="openCreate">{{ t.add }}</el-button>
          </el-space>
        </el-col>
      </el-row>

      <el-card shadow="never">
        <el-table :data="rows" v-loading="loading" size="default" style="width: 100%">
          <el-table-column prop="name" :label="t.identifier" min-width="220" />
          <el-table-column prop="host" :label="t.host" min-width="180" />
          <el-table-column prop="port" :label="t.port" width="90" />
          <el-table-column prop="username" :label="t.username" min-width="140" />
          <el-table-column :label="t.auth" width="120">
            <template #default="s">
              <el-tag v-if="s.row.pkey_path" type="success">Key</el-tag>
              <el-tag v-else-if="s.row.password" type="warning">Pwd</el-tag>
              <el-tag v-else type="info">None</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t.operation" width="200" align="right">
            <template #default="s">
              <el-space>
                <el-button size="small" icon="Edit" @click="openEdit(s.row)">{{ t.edit }}</el-button>
                <el-button size="small" icon="DocumentCopy" @click="openCopy(s.row)">{{ t.copy }}</el-button>
                <el-popconfirm :title="t.deleteConfirm" @confirm="del(s.row)">
                  <template #reference>
                    <el-button size="small" type="danger" icon="Delete">{{ t.delete }}</el-button>
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

      <el-dialog v-model="dialogVisible" :title="dialogMode === 'edit' ? t.editBastion : t.addBastion" width="520px" :close-on-click-modal="false">
        <el-form label-width="120px" size="default">
          <el-form-item :label="t.identifier">
            <el-input v-model="form.name" :disabled="dialogMode === 'edit'" :placeholder="t.identifier" />
          </el-form-item>
          <el-form-item :label="t.host" required>
            <el-input v-model="form.host" :disabled="dialogMode === 'edit'" placeholder="1.2.3.4" />
          </el-form-item>
          <el-form-item :label="t.port" required>
            <el-input-number v-model="form.port" :min="1" :max="65535" :disabled="dialogMode === 'edit'" style="width: 100%" />
          </el-form-item>
          <el-form-item :label="t.username" required>
            <el-input v-model="form.username" />
          </el-form-item>
          <el-form-item :label="t.password">
            <el-input v-model="form.password" show-password />
          </el-form-item>
          <el-form-item :label="t.privateKey">
            <el-input v-model="form.pkey_path" placeholder="~/.ssh/id_rsa" />
          </el-form-item>
          <el-form-item :label="t.passphrase">
            <el-input v-model="form.pkey_passphrase" show-password />
          </el-form-item>
        </el-form>

        <template #footer>
          <el-button @click="closeDialog">{{ t.cancel }}</el-button>
          <el-button type="primary" @click="submit">{{ t.confirm }}</el-button>
        </template>
      </el-dialog>
    </div>
  `,
};
