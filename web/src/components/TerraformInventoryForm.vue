<template>
  <v-form
    ref="form"
    lazy-validation
    v-model="formValid"
    v-if="item != null && keys != null"
  >
    <v-alert
      :value="formError"
      color="error"
      class="pb-2"
    >{{ formError }}</v-alert>

    <v-text-field
      v-model="item.inventory"
      :label="$t('uiWorkspaceName')"
      :rules="[v => !!v || $t('path_required')]"
      required
      :disabled="formSaving"
    ></v-text-field>

    <v-select
      v-model="item.ssh_key_id"
      :label="$t('uiSSHKeyForPrivateModules')"
      :rules="[v => !!v || $t('key_required')]"
      dense
      required
      :items="keys"
      item-value="id"
      item-text="name"
      :disabled="formSaving"
    ></v-select>

  </v-form>
</template>
<style>
</style>
<script>
/* eslint-disable import/no-extraneous-dependencies,import/extensions */

import ItemFormBase from '@/components/ItemFormBase';
import axios from 'axios';

export default {
  mixins: [ItemFormBase],

  props: {
    namePrefix: String,
    type: String,
    templateId: Number,
  },

  components: {
  },

  data() {
    return {
      keys: null,
    };
  },

  async created() {
    this.keys = (await axios({
      method: 'get',
      url: `/api/project/${this.projectId}/keys`,
      responseType: 'json',
    })).data;
  },

  methods: {
    async beforeSave() {
      this.item.name = this.namePrefix + this.item.inventory;
      this.item.type = this.type;
      this.item.template_id = this.templateId;
    },

    getItemsUrl() {
      return `/api/project/${this.projectId}/inventory`;
    },

    getSingleItemUrl() {
      return `/api/project/${this.projectId}/inventory/${this.itemId}`;
    },
  },
};
</script>
