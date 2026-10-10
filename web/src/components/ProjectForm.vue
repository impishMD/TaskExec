<template>
  <v-form ref="form" lazy-validation v-model="formValid" v-if="item != null">
    <v-alert :value="formError" color="error" class="pb-2">{{ formError }}</v-alert>

    <v-text-field
      v-model="item.name"
      :label="$t(projectNameTitle)"
      :rules="[(v) => !!v || $t('project_name_required')]"
      required
      :disabled="formSaving"
      data-testid="newProject-name"
      outlined
      dense
    ></v-text-field>

    <ProjectIconPicker v-model="item.icon" :name="item.name" :disabled="formSaving"
      @loading="iconLoading = $event" />

    <v-text-field
      v-model.number="item.max_parallel_tasks"
      :label="$t('maxNumberOfParallelTasksOptional')"
      :disabled="formSaving"
      :rules="[
        (v) => v == null || v === '' || Math.floor(v) === v || $t('mustBeInteger'),
        (v) => v == null || v === '' || v >= 0 || $t('mustBe0OrGreater'),
      ]"
      :hint="$t('uiShouldBe0OrGreater0Unlimited')"
      type="number"
      :step="1"
      outlined
      dense
    ></v-text-field>

    <p v-if="isNew" class="text-body-2 text--secondary">{{ $t('alertsAfterCreation') }}</p>

    <v-switch
      data-testid="newProject-demo"
      v-if="itemId === 'new' && !hideDemoSwitch"
      v-model="item.demo"
      :label="$t('uiDemo')"
      style="position: absolute; left: 24px; bottom: 15px"
      hide-details
    />
  </v-form>
</template>
<script>
import ItemFormBase from '@/components/ItemFormBase';
import ProjectIconPicker from '@/components/ProjectIconPicker.vue';

export default {
  mixins: [ItemFormBase],
  components: { ProjectIconPicker },
  data() { return { iconLoading: false }; },
  props: {
    projectNameTitle: {
      type: String,
      default: 'projectName',
    },
    hideDemoSwitch: Boolean,
  },
  methods: {
    getItemsUrl() {
      return '/api/projects';
    },
    getSingleItemUrl() {
      return `/api/project/${this.itemId}`;
    },
    beforeSave() {
      if (this.iconLoading) throw new Error(this.$t('projectIconLoading'));
      if (this.item.max_parallel_tasks === '') {
        this.item.max_parallel_tasks = 0;
      }
    },
  },
};
</script>
