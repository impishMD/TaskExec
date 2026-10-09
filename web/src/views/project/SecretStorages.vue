<template>
  <div v-if="items != null">
    <ObjectRefsDialog
      :object-title="$t('uiStorage')"
      :object-refs="itemRefs"
      :project-id="projectId"
      v-model="itemRefsDialog"
    />

    <YesNoDialog
      :title="$t('uiDeleteStorage')"
      :text="$t('uiDoYouReallyWantToDeleteThisStorage')"
      v-model="deleteItemDialog"
      @yes="deleteItem(itemId)"
    />

    <EditDialog
      v-model="editDialog"
      :save-button-text="itemId === 'new' ? $t('create') : $t('save')"
      :title="`${itemId === 'new' ? $t('uiNewStorage') : $t('edit')} ${storageTypeTitle(itemType)}`"
      :max-width="760"
      content-class="VaultStorageDialog"
      @save="loadItems()"
    >
      <template v-slot:form="{ onSave, onError, needSave, needReset }">
        <SecretStorageForm
          :project-id="projectId"
          :item-id="itemId"
          :item-type="itemType"
          @save="onSave"
          @error="onError"
          :need-save="needSave"
          :need-reset="needReset"
        />
      </template>
    </EditDialog>

    <v-toolbar flat>
      <v-app-bar-nav-icon @click="showDrawer()"></v-app-bar-nav-icon>
      <v-toolbar-title>{{ $t('keyStore') }}</v-toolbar-title>
      <v-spacer></v-spacer>

      <v-btn color="primary" v-if="can(USER_PERMISSIONS.manageProjectResources)"
        :disabled="!features.secret_storage_management"
        @click="itemType = 'vault'; editItem('new')">
        <v-icon left>mdi-plus</v-icon>{{ $t('uiNewStorage') }}
      </v-btn>
    </v-toolbar>

    <v-tabs class="pl-4">
      <v-tab key="keys" :to="`/project/${projectId}/keys`" data-testid="keystore-keys">
        {{ $t('uiKeys') }}
      </v-tab>

      <v-tab
        key="storages"
        :to="`/project/${projectId}/secret_storages`"
        data-testid="keystore-storages"
      >{{ $t('uiStorages') }}</v-tab>
    </v-tabs>

    <v-divider style="margin-top: -1px" />

    <v-data-table
      :headers="headers"
      :items="items"
      hide-default-footer
      class="mt-4"
      :items-per-page="Number.MAX_VALUE"
      style="max-width: calc(var(--breakpoint-xl) - var(--nav-drawer-width) - 200px); margin: auto"
    >
      <template v-slot:item.name="{ item }">
        <v-icon class="mr-3" small>
          {{ getIcon(item.type) }}
        </v-icon>

        <span class="mr-2">{{ item.name }}</span>

        <v-chip v-if="item.readonly" style="transform: translateY(-1px)" color="info" small>
          {{ $t('uiReadOnly') }}
        </v-chip>
      </template>

      <template v-slot:item.type="{ item }">
        {{ storageTypeTitle(item.type) }}
      </template>

      <template v-slot:item.actions="{ item }">
        <v-btn-toggle
          v-if="can(USER_PERMISSIONS.manageProjectResources)"
          dense :value-comparator="() => false"
        >
          <v-btn @click="askDeleteItem(item.id)">
            <v-icon>mdi-delete</v-icon>
          </v-btn>
          <v-btn
            @click="editItem(item.id); itemType = item.type"
            :disabled="item.type !== 'vault'"
          >
            <v-icon>mdi-pencil</v-icon>
          </v-btn>
        </v-btn-toggle>
      </template>
    </v-data-table>
  </div>
</template>

<script>
import DisplayLabelsMixin from '@/components/DisplayLabelsMixin';
import ItemListPageBase from '@/components/ItemListPageBase';
import SecretStorageForm from '@/components/SecretStorageForm.vue';

export default {
  components: { SecretStorageForm },
  mixins: [DisplayLabelsMixin, ItemListPageBase],
  data() {
    return {
      itemType: 'vault',
    };
  },

  props: {
    systemInfo: Object,
  },

  computed: {
    features() {
      return this.systemInfo?.features || {};
    },
  },

  methods: {

    getIcon(type) {
      switch (type) {
        case 'vault':
          return '$vuetify.icons.hashicorp_vault';
        case 'dvls':
          return '$vuetify.icons.dvls';
        case 'aws_sm':
          return '$vuetify.icons.aws_sm';
        case 'azure_kv':
          return '$vuetify.icons.azure_kv';
        default:
          return '';
      }
    },

    getHeaders() {
      return [
        {
          text: this.$i18n.t('name'),
          value: 'name',
          width: '60%',
        },
        {
          text: this.$i18n.t('type'),
          value: 'type',
          width: '40%',
        },
        {
          value: 'actions',
          sortable: false,
          width: '0%',
          align: 'end',
        },
      ];
    },
    getItemsUrl() {
      return `/api/project/${this.projectId}/secret_storages`;
    },
    getSingleItemUrl() {
      return `/api/project/${this.projectId}/secret_storages/${this.itemId}`;
    },
    getEventName() {
      return 'i-secret-storage';
    },
  },
};
</script>

<style lang="scss">
.VaultStorageDialog.v-dialog {
  max-height: calc(100vh - 48px);
  max-height: calc(100dvh - 48px);
  overflow: hidden;
  > .v-card {
    display: flex;
    flex-direction: column;
    max-height: inherit;
    > .v-card__text { overflow-y: auto; min-height: 0 !important; }
    > .v-card__title, > .v-card__actions { flex-shrink: 0; }
  }
}
.VaultStorageDialog.item-dialog .v-card__title {
  white-space: normal;
  word-break: normal;
  line-height: 1.4;
  padding-right: 64px;
}
.VaultStorageDialog .item-dialog__title-actions { top: 16px; }
</style>
