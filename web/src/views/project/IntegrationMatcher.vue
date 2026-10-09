<template xmlns:v-slot="http://www.w3.org/1999/XSL/Transform">
  <div v-if="items != null">
    <EditDialog
      v-model="editDialog"
      :save-button-text="itemId === 'new' ? $t('create') : $t('save')"
      :title="itemId === 'new' ? $t('newMatcher') : $t('editMatcher')"
      :max-width="450"
      :transition="false"
      @save="loadItems"
    >
      <template v-slot:form="{ onSave, onError, needSave, needReset }">
        <IntegrationMatcherForm
          :project-id="projectId"
          :integration-id="integrationId"
          :item-id="itemId"
          @save="onSave"
          @error="onError"
          :need-save="needSave"
          :need-reset="needReset"
        />
      </template>
    </EditDialog>

    <ObjectRefsDialog
      :object-title="$t('matcher')"
      :object-refs="itemRefs"
      :integration-id="integrationId"
      v-model="itemRefsDialog"
    />

    <YesNoDialog
      :title="$t('uiDeleteIntegrationMatcher')"
      :text="$t('askDeleteMatcher')"
      v-model="deleteItemDialog"
      @yes="deleteItem(itemId)"
    />

    <v-toolbar flat>
      <v-app-bar-nav-icon @click="showDrawer()"></v-app-bar-nav-icon>
      <v-toolbar-title>{{ $t('matcher') }}</v-toolbar-title>
      <v-spacer></v-spacer>
      <v-btn
        v-if="can(USER_PERMISSIONS.manageProjectResources)"
        color="primary"
        @click="editItem('new')"
        >{{ $t('newMatcher') }}</v-btn
      >
    </v-toolbar>

    <v-data-table :headers="headers" :items="items" class="mt-4" :items-per-page="Number.MAX_VALUE">
      <template v-slot:item.name="{ item }">
        {{ item.name }}
      </template>
      <template v-slot:item.match_type="{ item }">
        {{ integrationValueTitle(item.match_type) }}
      </template>
      <template v-slot:item.method="{ item }">
        {{ integrationValueTitle(item.method) }}
      </template>
      <template v-slot:item.body_data_type="{ item }">
        {{ integrationValueTitle(item.body_data_type) }}
      </template>
      <template v-slot:item.key="{ item }">
        <code>{{ item.key }}</code>
      </template>
      <template v-slot:item.value="{ item }">
        <code>{{ item.value }}</code>
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

import IntegrationMatcherForm from '@/components/IntegrationMatcherForm.vue';

export default {
  mixins: [DisplayLabelsMixin, ItemListPageBase],
  components: { IntegrationMatcherForm },
  computed: {
    // projectId() {
    //   if (/^-?\d+$/.test(this.$route.params.projectId)) {
    //     return parseInt(this.$route.params.projectId, 10);
    //   }
    //   return this.$route.params.projectId;
    // },
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
          text: this.$t('matchType'),
          value: 'match_type',
          sortable: true,
        },
        {
          text: this.$t('matchBodyDataType'),
          value: 'body_data_type',
          sortable: false,
        },
        {
          text: this.$t('matchKey'),
          value: 'key',
          sortable: true,
        },
        {
          text: this.$t('uiMethod'),
          value: 'method',
          sortable: true,
        },
        {
          text: this.$t('matchValue'),
          value: 'value',
          sortable: true,
        },
        {
          text: '',
          value: 'actions',
          sortable: false,
        },
      ];
    },
    getItemsUrl() {
      return `/api/project/${this.projectId}/integrations/${this.integrationId}/matchers`;
    },
    getSingleItemUrl() {
      return `/api/project/${this.projectId}/integrations/${this.integrationId}/matchers/${this.itemId}`;
    },
    getEventName() {
      return 'w-integration-matcher';
    },
  },
};
</script>
