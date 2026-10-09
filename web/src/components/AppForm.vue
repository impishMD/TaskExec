<template>
  <v-form
      ref="form"
      lazy-validation
      v-model="formValid"
      v-if="item != null"
  >
    <v-alert
        :value="formError"
        color="error"
        class="pb-2"
    >{{ formError }}
    </v-alert>

    <v-text-field
        v-model="id"
        :label="$t('uiID')"
        :rules="[v => !!v || $t('uiIDIsRequired')]"
        required
        :disabled="formSaving"
    ></v-text-field>

    <v-text-field
        v-model="item.icon"
        :label="$t('uiIcon')"
        :disabled="formSaving"
    ></v-text-field>

    <v-text-field
        v-model="item.title"
        :label="$t('name')"
        :disabled="formSaving"
    ></v-text-field>

    <v-text-field
        v-model="item.path"
        :label="$t('path')"
        :disabled="formSaving"
    ></v-text-field>

    <v-text-field
      type="number"
      v-model.number="item.priority"
      :label="$t('uiPriority')"
      :disabled="formSaving"
    ></v-text-field>

    <ArgsPicker style="margin-top: -10px;" :vars="item.args" @change="setArgs"/>

    <v-checkbox
        v-model="item.active"
        :label="$t('active2')"
    ></v-checkbox>

  </v-form>
</template>
<script>
import ItemFormBase from '@/components/ItemFormBase';
import ArgsPicker from './ArgsPicker.vue';

export default {
  components: { ArgsPicker },
  mixins: [ItemFormBase],

  computed: {
    isNew() {
      return this.itemId === '';
    },
  },

  data() {
    return {
      id: null,
    };
  },

  watch: {
    itemId() {
      this.id = this.itemId;
    },
  },

  methods: {
    setArgs(args) {
      this.item.args = args;
    },

    beforeLoadData() {
      if (!this.isNew) {
        this.id = this.itemId;
      }
    },

    afterReset() {
      this.id = null;
    },

    getItemsUrl() {
      return `/api/apps/${this.id}`;
    },

    getSingleItemUrl() {
      return `/api/apps/${this.id}`;
    },
  },
};
</script>
