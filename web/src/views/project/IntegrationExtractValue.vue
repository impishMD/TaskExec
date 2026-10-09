<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div v-if="items != null">
    <EditDialog
      v-model="editDialog"
      :save-button-text="itemId === 'new' ? $t('create') : $t('save')"
      :title="itemId === 'new' ? $t('newExtractedValue') : $t('editExtractedValue')"
      :max-width="450"
      :transition="false"
      @save="loadItems"
    >
      <template v-slot:form="{ onSave, onError, needSave, needReset }">
        <IntegrationExtractValueForm
          :projectId="projectId"
          :integration-id="integrationId"
          :item-id="itemId"
          :project-id="projectId"
          @save="onSave"
          @error="onError"
          :need-save="needSave"
          :need-reset="needReset"
        />
      </template>
    </EditDialog>

    <ObjectRefsDialog
      :object-title="$t('uiExtractValue')"
      :object-refs="itemRefs"
      :integration-id="integrationId"
      v-model="itemRefsDialog"
    />

    <YesNoDialog
      :title="$t('uiDeleteIntegrationExtractValue')"
      :text="$t('askDeleteExtractedValue')"
      v-model="deleteItemDialog"
      @yes="deleteItem(itemId)"
    />

    <v-toolbar flat>
      <v-app-bar-nav-icon @click="showDrawer()"></v-app-bar-nav-icon>
      <v-toolbar-title>{{ $t('uiExtractValue') }}</v-toolbar-title>
      <v-spacer></v-spacer>
      <v-btn
        v-if="can(USER_PERMISSIONS.manageProjectResources)"
        color="primary"
        @click="editItem('new')"
        >{{ $t('newExtractedValue') }}</v-btn
      >
    </v-toolbar>
    <v-data-table :headers="headers" :items="items" class="mt-4" :items-per-page="Number.MAX_VALUE">
      <template v-slot:item.name="{ item }"> {{ item.name }} {{ item.extractorId }} </template>
      <template v-slot:item.value_source="{ item }">
        {{ integrationValueTitle(item.value_source) }}
      </template>
      <template v-slot:item.body_data_type="{ item }">
        {{ integrationValueTitle(item.body_data_type) }}
      </template>
      <template v-slot:item.key="{ item }">
        <code>{{ item.key }}</code>
      </template>
      <template v-slot:item.variable="{ item }">
        <code>{{ item.variable }}</code>
      </template>
      <template v-slot:item.variable_type="{ item }">
        {{ integrationValueTitle(item.variable_type) }}
      </template>
      <template v-slot:item.actions="{ item }">
        <div style="white-space: nowrap">
          <v-btn icon class="mr-1" @click="askDeleteItem(item.id)">
            <v-icon>mdi-delete</v-icon>
          </v-btn>

          <v-btn icon class="mr-1" @click="editItem(item.id)">
            <v-icon>mdi-pencil</v-icon>
          </v-btn>
        </div>
      </template>
    </v-data-table>
  </div>
</template>
<script>
import DisplayLabelsMixin from '@/components/DisplayLabelsMixin';
import ItemListPageBase from '@/components/ItemListPageBase';

import IntegrationExtractValueForm from '@/components/IntegrationExtractValueForm.vue';

export default {
  mixins: [DisplayLabelsMixin, ItemListPageBase],
  components: { IntegrationExtractValueForm },

  computed: {
    integrationId() {
      if (/^-?\d+$/.test(this.$route.params.integrationId)) {
        return parseInt(this.$route.params.integrationId, 10);
      }
      return this.$route.params.integrationId;
    },
  },

  methods: {
    allowActions() {
      return true;
    },

    getHeaders() {
      return [
        {
          text: this.$t('name'),
          value: 'name',
          sortable: true,
        },
        {
          text: this.$t('extractedValueSource'),
          value: 'value_source',
          sortable: false,
        },
        // {
        //   text: 'Body Data Type',
        //   value: 'body_data_type',
        //   sortable: false,
        // },
        {
          text: this.$t('matchKey'),
          value: 'key',
          sortable: false,
        },
        {
          text: this.$t('uiVariableLabel'),
          value: 'variable',
          sortable: false,
        },
        {
          text: this.$t('uiVariableType'),
          value: 'variable_type',
          sortable: false,
        },
        {
          text: this.$t('actions'),
          value: 'actions',
          sortable: false,
        },
      ];
    },
    getItemsUrl() {
      return `/api/project/${this.projectId}/integrations/${this.integrationId}/values`;
    },
    getSingleItemUrl() {
      return `/api/project/${this.projectId}/integrations/${this.integrationId}/values/${this.itemId}`;
    },
    getEventName() {
      return 'w-integration-extract-value';
    },
  },
};
</script>
