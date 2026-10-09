<template>
  <div v-if="inventories != null && states != null" class="mt-8">

    <EditDialog
      v-model="editDialog"
      :save-button-text="aliasId === 'new' ? $t('create') : $t('save')"
      :title="`${aliasId === 'new' ? $t('nnew') : $t('edit')} ${$t('terraformBackendAlias')}`"
      :max-width="450"
      @save="loadAliases()"
    >
      <template v-slot:form="{ onSave, onError, needSave, needReset }">
        <TerraformAliasForm
          :project-id="template.project_id"
          :item-id="aliasId"
          :inventory-id="inventoryId"
          @save="onSave"
          @error="onError"
          :need-save="needSave"
          :need-reset="needReset"
        />
      </template>
    </EditDialog>

    <YesNoDialog
      :title="$t('deleteInventory')"
      :text="$t('askDeleteInv')"
      v-model="deleteInventoryDialog"
      @yes="deleteInventory()"
    />

    <EditDialog
      v-model="inventoryDialog"
      :save-button-text="itemId === 'new' ? $t('create') : $t('save')"
      :icon="getAppIcon(template.app)"
      :icon-color="getAppColor(template.app)"
      :title="`${$t('nnew')} ${APP_INVENTORY_TITLE[template.app]}`"
      :max-width="450"
      @save="onNewInventory"
    >
      <template v-slot:form="{ onSave, onError, needSave, needReset }">
        <TerraformInventoryForm
          :template-id="template.id"
          :project-id="template.project_id"
          :name-prefix="`${template.name} - `"
          :item-id="itemId"
          :type="`${template.app}-workspace`"
          @save="onSave"
          @error="onError"
          :need-save="needSave"
          :need-reset="needReset"
        />
      </template>
    </EditDialog>

    <EditDialog
      v-model="attachInventoryDialog"
      :save-button-text="$t('uiAttach')"
      :icon="getAppIcon(template.app)"
      :icon-color="getAppColor(template.app)"
      :max-width="450"
      :title="$t('uiChooseWorkspaceToAttach')"
      @save="attachInventory($event.itemId)"
    >
      <template v-slot:form="{ onSave, needSave, needReset }">
        <InventorySelectForm
          :app="template.app"
          :project-id="template.project_id"
          @save="onSave"
          :need-save="needSave"
          :need-reset="needReset"
        />
      </template>
    </EditDialog>

    <div
      class="px-4 py-3 CenterToScreen"
      style="max-width: 1000px; margin: auto;"
    >
      <div class="mb-6">
        <v-btn-toggle
          dense
          v-model="inventoryId"
          v-if="inventories.length > 0"
          mandatory
          style="overflow: auto"
          class="mb-2"
        >
          <v-btn
            v-for="inv in inventories"
            :key="inv.id"
            :value="inv.id"
            style="border-radius: 0"
          >
            {{ inv.inventory }}
            <v-icon
              dark
              v-if="inv.id === template.inventory_id"
              class="ml-1"
              color="success"
            >mdi-check
            </v-icon>
          </v-btn>
        </v-btn-toggle>

        <span v-else>{{ $t('uiNoWorkspaces') }}</span>

        <v-menu offset-y v-if="canManageResources">
          <template v-slot:activator="{ on, attrs }">
            <v-btn
              color="primary"
              dark
              fab
              small
              class="ml-3"
              v-bind="attrs"
              v-on="on"
            >
              <v-icon dark>mdi-plus</v-icon>
            </v-btn>
          </template>
          <v-list>
            <v-list-item @click="itemId = 'new'; inventoryDialog = true">
              <v-list-item-icon>
                <v-icon>mdi-pencil</v-icon>
              </v-list-item-icon>
              <v-list-item-title>{{ $t('uiNewWorkspace') }}</v-list-item-title>
            </v-list-item>
            <v-list-item @click="attachInventoryDialog = true">
              <v-list-item-icon>
                <v-icon>mdi-connection</v-icon>
              </v-list-item-icon>
              <v-list-item-title>{{ $t('uiAttachExistingWorkspace') }}</v-list-item-title>
            </v-list-item>
          </v-list>
        </v-menu>
      </div>

      <v-card
        style="background-color: var(--taskexec-surface);"
        class="TerraformWorkspace"
        v-if="inventories.length > 0"
      >

        <v-card-title
          class="d-flex justify-space-between align-center"
          style="font-weight: normal;"
        >

          <span>{{ $t('uiWorkspaceLabel') }} <strong>{{ inventory.inventory }}</strong></span>

          <div v-if="canManageResources" class="TerraformWorkspace__actions">

            <v-btn
              class="mr-4"
              :disabled="inventoryId === template.inventory_id"
              color="success"
              @click="setDefaultInventory()"
            >{{ $t('uiMakeDefault') }}</v-btn>

            <v-btn
              class="mr-4"
              color="primary"
              :disabled="inventoryId === template.inventory_id"
              @click="detachInventory()"
            >{{ $t('uiDetach') }}</v-btn>

            <v-btn
              icon
              class="mr-2"
              color="error"
              :disabled="inventoryId === template.inventory_id"
              @click="deleteInventoryDialog = true;"
            >
              <v-icon>mdi-delete</v-icon>
            </v-btn>

            <v-btn
              icon
              @click="itemId = inventoryId; inventoryDialog = true;"
            >
              <v-icon>mdi-pencil</v-icon>
            </v-btn>
          </div>

        </v-card-title>

        <v-divider />
        <v-card-text>

          <h3>{{ $t('aliases') }}</h3>
          <div class="mb-6">
            {{ $t('uiUniqueEndpointsAliasesToAccessYourTerraformHTTPBackend') }}
          </div>

          <div v-for="alias of (aliases || [])" :key="alias.id" class="TerraformWorkspace__alias">
            <code>{{ alias.url }}</code>
            <CopyClipboardButton
              :text="alias.url"
              :success-message="$t('aliasUrlCopied')"
            />
            <v-btn v-if="canManageResources" icon @click="editAlias(alias.id)">
              <v-icon>mdi-pencil</v-icon>
            </v-btn>
            <v-btn v-if="canManageResources" icon @click="deleteAlias(alias.id)">
              <v-icon>mdi-delete</v-icon>
            </v-btn>
          </div>

          <v-btn
            v-if="canManageResources"
            data-testid="terraform-add-alias"
            color="primary"
            @click="addAlias()"
            :disabled="!features.terraform_backend"
            class="mb-8"
          >
            {{ aliases == null ? $t('LoadAlias') : $t('AddAlias') }}
          </v-btn>

          <v-divider class="mb-4" />

          <h3>{{ $t('uiStateHistory') }}</h3>
          <div class="mb-6">{{ $t('uiChronologicalHistoryOfTheWorkspaceStateChanges') }}</div>

          <v-data-table
            style="
              background: transparent;
            "
            v-if="features.terraform_backend"
            :headers="headers"
            :items="states"
            :mobile-breakpoint="0"
            :footer-props="{ itemsPerPageOptions: [20] }"
            single-expand
            :show-expand="canManageResources"
            data-testid="terraform-state-history"
            class="mt-4 TaskListTable TerraformStateTable"
          >
            <template v-slot:item.id="{ item }">
              #{{ item.id }}
            </template>

            <template v-slot:item.task_id="{ item }">
              <TaskLink
                v-if="item.task_id"
                :task-id="item.task_id"
                :label="'#' + item.task_id"
              />
              <div v-else>&mdash;</div>
            </template>

            <template v-slot:item.status="{ item }">
              <TaskStatus :status="item.status" />
            </template>

            <template v-slot:item.created="{ item }">
              {{ item.created | formatDate }}
            </template>

            <template v-slot:expanded-item="{ headers, item }">
              <td
                :colspan="headers.length"
                style="max-width: 400px"
              >
                <TerraformStateView
                  class="mb-1"
                  :project-id="template.project_id"
                  :inventory-id="inventoryId"
                  :state-id="item.id"
                />
              </td>
            </template>
          </v-data-table>

          <v-container v-else>
            <div style="text-align: center; color: grey;">{{ $t('uiNoStateAvailable') }}</div>
          </v-container>

        </v-card-text>
      </v-card>

    </div>

  </div>
</template>
<style lang="scss">
.TerraformWorkspace {
  .v-card__title { gap: 12px; word-break: normal; }
  &__actions { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
  &__alias {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-bottom: 12px;
    code { overflow-wrap: anywhere; min-width: 0; }
  }
}
.taskexec-app.v-application .taskexec-workspace .v-data-table.TerraformStateTable {

  @media (max-width: 600px) {
    table { table-layout: fixed; }
    > .v-data-table__wrapper > table {
      > thead > tr > th, > tbody > tr > td {
        padding-left: 8px !important;
        padding-right: 8px !important;
        white-space: normal;
        overflow-wrap: anywhere;
      }
      > thead > tr > th span { white-space: normal; overflow-wrap: anywhere; }
      > thead > tr > th:first-child, > tbody > tr > td:first-child { padding-left: 8px !important; }
      > thead > tr > th:last-child, > tbody > tr > td:last-child { padding-right: 8px !important; }
      > thead > tr > th:nth-child(1) { width: 48px !important; min-width: 48px; }
      > thead > tr > th:nth-child(2) { width: 48px; }
      > thead > tr > th:nth-child(3) { width: 80px; }
    }
  }

  .v-data-table__wrapper {
    padding-left: 0 !important;
    padding-right: 0 !important;
  }

  .v-data-footer {
    margin-left: 0 !important;
    margin-right: 0 !important;
  }
}
</style>
<script>

import axios from 'axios';
import TerraformInventoryForm from '@/components/TerraformInventoryForm.vue';
import EditDialog from '@/components/EditDialog.vue';
import { APP_INVENTORY_TITLE, USER_PERMISSIONS } from '@/lib/constants';
import AppsMixin from '@/components/AppsMixin';
import YesNoDialog from '@/components/YesNoDialog.vue';
import InventorySelectForm from '@/components/InventorySelectForm.vue';
import TerraformAliasForm from '@/components/TerraformAliasForm.vue';
import TaskStatus from '@/components/TaskStatus.vue';
import TaskLink from '@/components/TaskLink.vue';
import TerraformStateView from '@/components/TerraformStateView.vue';
import CopyClipboardButton from '@/components/CopyClipboardButton.vue';

export default {
  mixins: [AppsMixin],
  computed: {
    canManageResources() {
      // State may contain secrets shared by multiple templates in the project.
      // A grant on this template alone does not grant access to their state.
      // eslint-disable-next-line no-bitwise
      const canManage = (this.projectPermissions & USER_PERMISSIONS.manageProjectResources) !== 0;
      return this.isAdmin || canManage;
    },
    APP_INVENTORY_TITLE() {
      return Object.fromEntries(Object.entries(APP_INVENTORY_TITLE).map(([id, title]) => [
        id, this.$t(id === 'ansible' ? 'appInventory' : 'appWorkspace', { app: title.split(' ')[0] }),
      ]));
    },
  },

  components: {
    CopyClipboardButton,
    TerraformStateView,
    TaskLink,
    TaskStatus,
    TerraformAliasForm,
    InventorySelectForm,
    YesNoDialog,
    EditDialog,
    TerraformInventoryForm,
  },

  props: {
    template: Object,
    features: Object,
    isAdmin: Boolean,
    projectPermissions: Number,
  },

  watch: {
    async inventoryId() {
      this.inventory = (this.inventories || []).find((inv) => inv.id === this.inventoryId);
      await this.loadAliases();
      await this.loadStates();
    },
  },

  data() {
    return {
      aliases: [],
      states: null,
      inventory: null,
      inventories: null,
      inventoryDialog: null,
      inventoryId: null,
      deleteInventoryDialog: null,
      attachInventoryDialog: null,
      aliasId: null,
      editDialog: null,
      headers: [
        {
          text: this.$t('uiID'),
          value: 'id',
          sortable: false,
        },
        {
          text: this.$t('taskId'),
          value: 'task_id',
          sortable: false,
        },
        {
          text: this.$t('uiCreated'),
          value: 'created',
          sortable: false,
        },
      ],
      itemId: null,
    };
  },

  async created() {
    await this.loadInventories();
    if (this.inventoryId == null) {
      this.states = [];
    }
  },

  methods: {

    async setDefaultInventory() {
      await axios({
        method: 'post',
        url: `/api/project/${this.template.project_id}/templates/${this.template.id}/inventory/${this.inventoryId}/set_default`,
      });
      this.$emit('update-template', {});
    },

    async detachInventory() {
      await axios({
        method: 'post',
        url: `/api/project/${this.template.project_id}/templates/${this.template.id}/inventory/${this.inventoryId}/detach`,
      });
      await this.loadInventories();
    },

    async deleteInventory() {
      await axios({
        method: 'delete',
        url: `/api/project/${this.template.project_id}/inventory/${this.inventoryId}`,
      });
      await this.loadInventories();
    },

    async onNewInventory(e) {
      await this.loadInventories();
      this.inventoryId = e.item.id;
    },

    async attachInventory(inventoryId) {
      await axios({
        method: 'post',
        url: `/api/project/${this.template.project_id}/templates/${this.template.id}/inventory/${inventoryId}/attach`,
      });
      await this.loadInventories();
      this.inventoryId = inventoryId;
    },

    async loadStates() {
      if (!this.inventoryId) {
        this.states = [];
        return;
      }
      this.states = (await axios.get(`/api/project/${this.template.project_id}/inventory/${this.inventoryId}/terraform/states`)).data;
    },

    async loadInventories() {
      this.inventories = (await axios({
        url: `/api/project/${this.template.project_id}/inventory?template_id=${this.template.id}`,
        responseType: 'json',
      })).data;

      if (
        (this.inventoryId == null
          || !this.inventories.some((inv) => inv.id === this.inventoryId))
        && this.inventories.length > 0) {
        this.inventoryId = this.inventories.some((inv) => inv.id === this.template.inventory_id)
          ? this.template.inventory_id : this.inventories[0].id;
      }
    },

    async loadAliases() {
      if (!this.inventoryId) {
        this.aliases = [];
        return;
      }
      try {
        this.aliases = (await axios({
          url: `/api/project/${this.template.project_id}/inventory/${this.inventoryId}/terraform/aliases`,
          responseType: 'json',
        })).data;
      } catch {
        this.aliases = null;
      }
    },

    async deleteAlias(alias) {
      await axios({
        method: 'delete',
        url: `/api/project/${this.template.project_id}/inventory/${this.inventoryId}/terraform/aliases/${alias}`,
      });
      await this.loadAliases();
    },

    editAlias(alias) {
      this.aliasId = alias;
      this.editDialog = true;
    },

    async addAlias() {
      this.aliasId = 'new';
      this.editDialog = true;
    },
  },
};
</script>
