<template>
  <v-form ref="form" lazy-validation v-model="formValid" v-if="item">
    <v-alert :value="formError" color="error" class="pb-2">{{ formError }}</v-alert>

    <v-text-field
      v-model="item.name"
      :label="$t('name')"
      :rules="[(v) => !!v || $t('name_required')]"
      required
      :disabled="formSaving"
    ></v-text-field>
    <v-row>
      <v-col cols="12" md="12" class="pb-0">
        <div class="ml-4 mr-4 mt-6">
          <v-select
            v-model="item.value_source"
            :label="$t('uiSourceOfTheValue')"
            :items="valueSources"
            item-value="id"
            item-text="text"
            :rules="[(v) => !!v || $t('uiValueSourceRequired')]"
            outlined
            dense
            required
            :disabled="formSaving"
          >
          </v-select>
          <v-select
            v-model="item.body_data_type"
            :label="$t('uiDataTypeOfBody')"
            v-if="item.value_source == 'body'"
            :items="bodyDataTypes"
            item-value="id"
            item-text="text"
            :rules="[(v) => !!v || $t('uiBodyTypeRequired')]"
            outlined
            dense
            required
            :disabled="formSaving"
          >
          </v-select>
          <v-text-field
            v-model="item.key"
            :label="$t('uiKey')"
            :rules="[(v) => !!v || $t('key_required')]"
            outlined
            dense
            required
            :disabled="formSaving"
          >
          </v-text-field>
          <v-select
            v-model="item.variable_type"
            :label="$t('uiVariableUsage')"
            :items="variableTypes"
            item-value="id"
            item-text="text"
            :rules="[(v) => !!v || $t('uiVariableTypeRequired')]"
            outlined
            dense
            required
            :disabled="formSaving"
          >
          </v-select>
          <v-text-field
            v-model="item.variable"
            :label="$t('uiVariable')"
            :rules="[(v) => !!v || $t('variableRequired')]"
            outlined
            dense
            required
            :disabled="formSaving"
          ></v-text-field>
        </div>
      </v-col>
    </v-row>
  </v-form>
</template>
<script>
import ItemFormBase from '@/components/ItemFormBase';
import IntegrationExtractorChildValueFormBase from './IntegrationExtractorChildValueFormBase';
import { EXTRACT_VALUE_TYPE_ICONS, EXTRACT_VALUE_TYPE_TITLES } from '../lib/constants';

export default {
  mixins: [ItemFormBase, IntegrationExtractorChildValueFormBase],
  props: {
    integrationId: Number,
  },
  data() {
    return {
      EXTRACT_VALUE_TYPE_ICONS,
      EXTRACT_VALUE_TYPE_TITLES,
      valueSources: [
        {
          id: 'body',
          text: this.$t('uiBody'),
        },
        {
          id: 'header',
          text: this.$t('uiHeader'),
        },
      ],
      bodyDataTypes: [
        {
          id: 'json',
          text: 'JSON',
        },
        {
          id: 'string',
          text: this.$t('uiString'),
        },
      ],
      variableTypes: [
        {
          id: 'environment',
          text: this.$t('uiVariables'),
        },
        {
          id: 'task',
          text: this.$t('uiTaskParams'),
        },
      ],
    };
  },
  computed: {
    // integrationId() {
    //   if (/^-?\d+$/.test(this.$route.params.integrationId)) {
    //     return parseInt(this.$route.params.integrationId, 10);
    //   }
    //   return this.$route.params.integrationId;
    // },
  },
  methods: {
    getItemsUrl() {
      return `/api/project/${this.projectId}/integrations/${this.integrationId}/values`;
    },
    getSingleItemUrl() {
      return `/api/project/${this.projectId}/integrations/${this.integrationId}/values/${this.itemId}`;
    },
  },
};
</script>
