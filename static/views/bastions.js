import { apiDELETE, apiGET, apiPOST, apiPUT } from "../app/api.js";

const { ref, inject, onMounted } = Vue;

export default {
  name: "ViewBastions",
  setup() {
    const t = inject("t");
    const ElMessage = ElementPlus.ElMessage;

    const bastions = ref([]);
    const isEditingBastion = ref(false);
    const editingBastionId = ref(null);

    const defaultBastion = {
      host: "",
      port: 22,
      username: "root",
      password: "",
      pkey_path: "",
      pkey_passphrase: "",
    };
    const bastionForm = ref({ ...defaultBastion });

    const loadBastions = async () => {
      bastions.value = await apiGET("/bastions");
    };

    const resetBastionForm = () => {
      bastionForm.value = { ...defaultBastion };
      isEditingBastion.value = false;
      editingBastionId.value = null;
    };

    const submitBastion = async () => {
      try {
        if (isEditingBastion.value && editingBastionId.value) {
          await apiPUT(`/bastions/${editingBastionId.value}`, bastionForm.value);
          ElMessage.success(t.value.editSuccess);
        } else {
          await apiPOST("/bastions", bastionForm.value);
          ElMessage.success(t.value.addSuccess);
        }
        resetBastionForm();
        await loadBastions();
      } catch (e) {
        ElMessage.error(e.message);
      }
    };

    const editBastion = (row) => {
      bastionForm.value = JSON.parse(JSON.stringify(row));
      isEditingBastion.value = true;
      editingBastionId.value = row.id;
    };

    const copyBastion = (row) => {
      bastionForm.value = JSON.parse(JSON.stringify(row));
      delete bastionForm.value.id;
      isEditingBastion.value = false;
      editingBastionId.value = null;
      ElMessage.info(t.value.configCopied);
    };

    const delBastion = async (id) => {
      try {
        await apiDELETE(`/bastions/${id}`);
        if (editingBastionId.value === id) resetBastionForm();
        await loadBastions();
      } catch (e) {
        ElMessage.error(e.message);
      }
    };

    onMounted(() => {
      loadBastions();
    });

    return {
      t,
      bastions,
      bastionForm,
      isEditingBastion,
      submitBastion,
      resetBastionForm,
      editBastion,
      copyBastion,
      delBastion,
    };
  },
  template: `
    <div>
      <el-row :gutter="24">
        <el-col :span="6">
          <el-card shadow="never" :class="{ 'editing-card': isEditingBastion }">
            <template #header>
              <b>{{ isEditingBastion ? t.editBastion : t.addBastion }}</b>
              <el-button
                v-if="isEditingBastion"
                size="small"
                style="float: right"
                @click="resetBastionForm"
                >{{ t.cancel }}</el-button
              >
            </template>
            <el-form :model="bastionForm" label-width="60px" size="small">
              <el-form-item :label="t.host">
                <el-input
                  v-model="bastionForm.host"
                  placeholder="1.2.3.4"
                  :disabled="isEditingBastion"
                />
              </el-form-item>
              <el-form-item :label="t.port">
                <el-input-number
                  v-model="bastionForm.port"
                  :min="1"
                  :max="65535"
                  style="width: 100%"
                  :disabled="isEditingBastion"
                />
              </el-form-item>
              <el-form-item :label="t.username">
                <el-input v-model="bastionForm.username" />
              </el-form-item>
              <el-form-item :label="t.password">
                <el-input v-model="bastionForm.password" show-password />
              </el-form-item>
              <el-form-item :label="t.privateKey">
                <el-input
                  v-model="bastionForm.pkey_path"
                  placeholder="~/.ssh/id_rsa"
                />
              </el-form-item>
              <el-form-item :label="t.passphrase">
                <el-input v-model="bastionForm.pkey_passphrase" show-password />
              </el-form-item>
              <el-form-item>
                <el-button
                  type="primary"
                  @click="submitBastion"
                  :disabled="!bastionForm.host"
                >
                  {{ isEditingBastion ? t.saveChanges : t.add }}
                </el-button>
                <el-button @click="resetBastionForm" v-if="!isEditingBastion"
                  >{{ t.reset }}</el-button
                >
              </el-form-item>
            </el-form>
          </el-card>
        </el-col>

        <el-col :span="18">
          <el-card shadow="never">
            <template #header><b>{{ t.bastionList }}</b></template>
            <el-table :data="bastions" size="small" style="width: 100%">
              <el-table-column
                prop="name"
                :label="t.identifier"
                width="200"
              />
              <el-table-column prop="host" :label="t.address" />
              <el-table-column prop="username" :label="t.user" width="200" />
              <el-table-column :label="t.auth" width="120">
                <template #default="s">
                  <el-tag v-if="s.row.pkey_path" size="small" type="success"
                    >Key</el-tag
                  >
                  <el-tag v-else-if="s.row.password" size="small" type="warning"
                    >Pwd</el-tag
                  >
                  <el-tag v-else size="small" type="info">None</el-tag>
                </template>
              </el-table-column>
              <el-table-column :label="t.operation" width="200">
                <template #default="s">
                  <div style="display: flex; gap: 5px">
                    <el-tooltip :content="t.editConfig" placement="top">
                      <el-button
                        size="small"
                        circle
                        icon="Edit"
                        @click="editBastion(s.row)"
                      />
                    </el-tooltip>
                    <el-tooltip :content="t.copyConfig" placement="top">
                      <el-button
                        size="small"
                        circle
                        icon="DocumentCopy"
                        @click="copyBastion(s.row)"
                      />
                    </el-tooltip>
                    <el-popconfirm
                      :title="t.deleteConfirm"
                      @confirm="delBastion(s.row.id)"
                    >
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
